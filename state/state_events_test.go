package state

import (
	"slices"
	"testing"

	"github.com/ayn2op/arikawa/v3/api"
	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/gateway"
	"github.com/ayn2op/arikawa/v3/session"
	"github.com/ayn2op/arikawa/v3/state/store/defaultstore"
	"github.com/ayn2op/arikawa/v3/utils/handler"
)

func TestStateOnEvent(t *testing.T) {
	// threads returns the threads left in guild 1 after event, which started with 11 and 12 in channel 10 and 21 in channel 20.
	threads := func(t *testing.T, event any) []discord.ChannelID {
		t.Helper()
		cabinet := defaultstore.New()
		for _, channel := range []discord.Channel{
			{ID: 10, GuildID: 1},
			{ID: 20, GuildID: 1},
			{ID: 11, GuildID: 1, ParentID: 10, Type: discord.GuildPublicThread},
			{ID: 12, GuildID: 1, ParentID: 10, Type: discord.GuildPrivateThread},
			{ID: 21, GuildID: 1, ParentID: 20, Type: discord.GuildPublicThread},
		} {
			if err := cabinet.ChannelSet(&channel, false); err != nil {
				t.Fatal(err)
			}
		}

		state := NewFromSession(session.NewCustom(gateway.DefaultIdentifier("token"), api.NewClient("token"), handler.New()), cabinet)
		state.onEvent(event)

		channels, err := cabinet.Channels(1)
		if err != nil {
			t.Fatal(err)
		}

		var ids []discord.ChannelID
		for _, channel := range channels {
			if channel.Type.IsThread() {
				ids = append(ids, channel.ID)
			}
		}

		// Removing from the store does not keep its order.
		slices.Sort(ids)
		return ids
	}

	thread := func(id, parentID discord.ChannelID, archived bool) discord.Channel {
		return discord.Channel{ID: id, GuildID: 1, ParentID: parentID, Type: discord.GuildPublicThread, ThreadMetadata: &discord.ThreadMetadata{Archived: archived}}
	}

	for name, tt := range map[string]struct {
		event any
		want  []discord.ChannelID
	}{
		"a sync of a channel drops its threads that are not listed": {
			&gateway.ThreadListSyncEvent{GuildID: 1, ChannelIDs: []discord.ChannelID{10}, Threads: []discord.Channel{thread(11, 10, false)}},
			[]discord.ChannelID{11, 21},
		},
		"a sync of the guild drops every thread that is not listed": {
			&gateway.ThreadListSyncEvent{GuildID: 1, Threads: []discord.Channel{thread(21, 20, false)}},
			[]discord.ChannelID{21},
		},
		"a deleted channel takes its threads with it": {
			&gateway.ChannelDeleteEvent{Channel: discord.Channel{ID: 10, GuildID: 1}},
			[]discord.ChannelID{21},
		},
		"an archived thread is dropped": {
			&gateway.ThreadUpdateEvent{Channel: thread(11, 10, true)},
			[]discord.ChannelID{12, 21},
		},
		"an updated thread is kept": {
			&gateway.ThreadUpdateEvent{Channel: thread(11, 10, false)},
			[]discord.ChannelID{11, 12, 21},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := threads(t, tt.event); !slices.Equal(got, tt.want) {
				t.Fatalf("threads = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("the thread member of the current user", func(t *testing.T) {
		// member returns the current user 1's member of thread 11 after events, which started with them in it, muted.
		member := func(t *testing.T, events ...any) *discord.ThreadMember {
			t.Helper()
			cabinet := defaultstore.New()
			if err := cabinet.MyselfSet(discord.User{ID: 1}, false); err != nil {
				t.Fatal(err)
			}

			muted := &discord.ThreadMember{ID: 11, UserID: 1, Muted: true}
			if err := cabinet.ChannelSet(&discord.Channel{ID: 11, GuildID: 1, ParentID: 10, Type: discord.GuildPublicThread, ThreadMember: muted}, false); err != nil {
				t.Fatal(err)
			}

			state := NewFromSession(session.NewCustom(gateway.DefaultIdentifier("token"), api.NewClient("token"), handler.New()), cabinet)
			for _, event := range events {
				state.onEvent(event)
			}

			thread, err := cabinet.Channel(11)
			if err != nil {
				t.Fatal(err)
			}

			return thread.ThreadMember
		}

		for name, tt := range map[string]struct {
			events []any
			joined bool
		}{
			"an update keeps it": {
				[]any{&gateway.ThreadUpdateEvent{Channel: thread(11, 10, false)}},
				true,
			},
			"a sync without it leaves the thread": {
				[]any{&gateway.ThreadListSyncEvent{GuildID: 1, ChannelIDs: []discord.ChannelID{10}, Threads: []discord.Channel{thread(11, 10, false)}}},
				false,
			},
			"a sync with it joins the thread": {
				[]any{
					&gateway.ThreadMembersUpdateEvent{ID: 11, RemovedMemberIDs: []discord.UserID{1}},
					&gateway.ThreadListSyncEvent{GuildID: 1, Threads: []discord.Channel{thread(11, 10, false)}, Members: []discord.ThreadMember{{ID: 11, UserID: 1}}},
				},
				true,
			},
			"being removed leaves the thread": {
				[]any{&gateway.ThreadMembersUpdateEvent{ID: 11, RemovedMemberIDs: []discord.UserID{1}}},
				false,
			},
			"someone else being removed keeps it": {
				[]any{&gateway.ThreadMembersUpdateEvent{ID: 11, RemovedMemberIDs: []discord.UserID{2}}},
				true,
			},
		} {
			t.Run(name, func(t *testing.T) {
				if got := member(t, tt.events...); (got != nil) != tt.joined {
					t.Fatalf("member = %+v, want joined %v", got, tt.joined)
				}
			})
		}

		t.Run("an update of the member replaces it", func(t *testing.T) {
			if got := member(t, &gateway.ThreadMemberUpdateEvent{ThreadMember: discord.ThreadMember{ID: 11, UserID: 1}}); got == nil || got.Muted {
				t.Fatalf("member = %+v, want an unmuted one", got)
			}
		})
	})
}
