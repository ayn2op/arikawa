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
}
