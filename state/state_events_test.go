package state

import (
	"testing"

	"github.com/ayn2op/arikawa/v3/api"
	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/gateway"
	"github.com/ayn2op/arikawa/v3/session"
	"github.com/ayn2op/arikawa/v3/state/store/defaultstore"
	"github.com/ayn2op/arikawa/v3/utils/handler"
)

func TestStateOnEvent(t *testing.T) {
	thread := func(id, parentID discord.ChannelID, archived bool) discord.Channel {
		return discord.Channel{ID: id, GuildID: 1, ParentID: parentID, Type: discord.GuildPublicThread, ThreadMetadata: &discord.ThreadMetadata{Archived: archived}}
	}

	t.Run("a deleted channel takes its threads with it", func(t *testing.T) {
		cabinet := defaultstore.New()
		for _, channel := range []discord.Channel{{ID: 10, GuildID: 1}, thread(11, 10, false), thread(21, 20, false)} {
			if err := cabinet.ChannelSet(&channel, false); err != nil {
				t.Fatal(err)
			}
		}

		state := NewFromSession(session.NewCustom(gateway.DefaultIdentifier("token"), api.NewClient("token"), handler.New()), cabinet)
		state.onEvent(&gateway.ChannelDeleteEvent{ID: 10, GuildID: 1})

		if _, err := cabinet.Channel(11); err == nil {
			t.Fatal("the thread of the deleted channel is still there")
		}

		if _, err := cabinet.Channel(21); err != nil {
			t.Fatalf("the thread of another channel is gone: %v", err)
		}
	})

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

		t.Run("an update of the members sets the member count", func(t *testing.T) {
			cabinet := defaultstore.New()
			if err := cabinet.ChannelSet(&discord.Channel{ID: 11, GuildID: 1, Type: discord.GuildPublicThread}, false); err != nil {
				t.Fatal(err)
			}

			state := NewFromSession(session.NewCustom(gateway.DefaultIdentifier("token"), api.NewClient("token"), handler.New()), cabinet)
			state.onEvent(&gateway.ThreadMembersUpdateEvent{ID: 11, GuildID: 1, MemberCount: 3})

			if thread, err := cabinet.Channel(11); err != nil || thread.MemberCount != 3 {
				t.Fatalf("thread = %+v, %v, want 3 members", thread, err)
			}
		})

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
			if got := member(t, &gateway.ThreadMemberUpdateEvent{ID: 11, UserID: 1}); got == nil || got.Muted {
				t.Fatalf("member = %+v, want an unmuted one", got)
			}
		})
	})
}
