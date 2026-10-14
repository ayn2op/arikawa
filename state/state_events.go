package state

import (
	"fmt"
	"slices"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/gateway"
	"github.com/ayn2op/arikawa/v3/state/store"
)

func (s *State) hookSession() {
	s.Session.AddSyncHandler(func(event any) {
		// Call the pre-handler before the state handler.
		if s.PreHandler != nil {
			s.PreHandler.Call(event)
		}

		// Run the state handler.
		s.onEvent(event)

		switch event := event.(type) {
		case *gateway.ReadyEvent:
			s.Handler.Call(event)
			s.handleReady(event)
		case *gateway.GuildCreateEvent:
			s.Handler.Call(event)
			s.handleGuildCreate(event)
		case *gateway.GuildDeleteEvent:
			s.Handler.Call(event)
			s.handleGuildDelete(event)

		// https://github.com/discord/discord-api-docs/commit/01665c4
		case *gateway.MessageCreateEvent:
			if event.Member != nil {
				event.Member.User = event.Author
			}
			s.Handler.Call(event)

		case *gateway.MessageUpdateEvent:
			if event.Member != nil {
				event.Member.User = event.Author
			}
			s.Handler.Call(event)

		default:
			s.Handler.Call(event)
		}
	})
}

func (s *State) onEvent(iface any) {
	switch ev := iface.(type) {
	case *gateway.ReadyEvent:
		// Acquire the ready mutex, but since we're only writing the value and
		// not anything in it, we should be fine.
		s.readyMu.Lock()
		s.ready = *ev
		s.readyMu.Unlock()

		// Reset the store before proceeding.
		if err := s.Cabinet.Reset(); err != nil {
			s.stateErr(err, "failed to reset state in Ready")
		}

		// Handle guilds
		for i := range ev.Guilds {
			s.batchLog(storeGuildCreate(s.Cabinet, &ev.Guilds[i]))
		}

		// Handle guild presences
		for i, presence := range ev.Presences {
			if err := s.Cabinet.PresenceSet(presence.GuildID, &ev.Presences[i], false); err != nil {
				s.stateErr(err, "failed to set presence in Ready")
			}
		}

		// Handle private channels
		for i := range ev.PrivateChannels {
			if err := s.Cabinet.ChannelSet(&ev.PrivateChannels[i], false); err != nil {
				s.stateErr(err, "failed to set channel in Ready")
			}
		}

		// Handle user
		if err := s.Cabinet.MyselfSet(ev.User, false); err != nil {
			s.stateErr(err, "failed to set self in Ready")
		}
		if err := s.Cabinet.UserSet(ev.Users...); err != nil {
			s.stateErr(err, "failed to set users in Ready")
		}
		s.storeMerged(ev.Guilds, ev.MergedMembers, ev.MergedPresences)

	case *gateway.ReadySupplementalEvent:
		ready := s.Ready()

		// Handle guilds
		for _, guild := range ev.Guilds {
			// Handle guild voice states
			for i := range guild.VoiceStates {
				v := &guild.VoiceStates[i]
				if err := s.Cabinet.VoiceStateSet(guild.ID, v, false); err != nil {
					s.stateErr(err, "failed to set guild voice state in Ready Supplemental")
				}
			}
		}

		s.storeMerged(ready.Guilds, ev.MergedMembers, ev.MergedPresences)

		for i := range ev.LazyPrivateChannels {
			for j := range ev.LazyPrivateChannels[i].DMRecipients {
				ev.LazyPrivateChannels[i].DMRecipients[j] = s.cachedUser(ev.LazyPrivateChannels[i].DMRecipients[j].ID)
			}
			if err := s.Cabinet.ChannelSet(&ev.LazyPrivateChannels[i], false); err != nil {
				s.stateErr(err, "failed to set channel in Ready Supplemental")
			}
		}

	case *gateway.GuildCreateEvent:
		s.batchLog(storeGuildCreate(s.Cabinet, ev))

	case *gateway.GuildUpdateEvent:
		if err := s.Cabinet.GuildSet(&ev.Guild, true); err != nil {
			s.stateErr(err, "failed to update guild in state")
		}

	case *gateway.GuildDeleteEvent:
		if err := s.Cabinet.GuildRemove(ev.ID); err != nil && !ev.Unavailable {
			s.stateErr(err, "failed to delete guild in state")
		}

	case *gateway.GuildMemberAddEvent:
		if err := s.Cabinet.MemberSet(ev.GuildID, &ev.Member, false); err != nil {
			s.stateErr(err, "failed to add a member in state")
		}

	case *gateway.GuildMemberUpdateEvent:
		m, err := s.Cabinet.Member(ev.GuildID, ev.User.ID)
		if err != nil {
			// We can't do much here.
			m = &discord.Member{}
		}

		// Update available fields from ev into m
		ev.UpdateMember(m)

		if err := s.Cabinet.MemberSet(ev.GuildID, m, true); err != nil {
			s.stateErr(err, "failed to update a member in state")
		}

	case *gateway.GuildMemberRemoveEvent:
		if err := s.Cabinet.MemberRemove(ev.GuildID, ev.User.ID); err != nil {
			s.stateErr(err, "failed to remove a member in state")
		}

	case *gateway.GuildMembersChunkEvent:
		for i := range ev.Members {
			if err := s.Cabinet.MemberSet(ev.GuildID, &ev.Members[i], false); err != nil {
				s.stateErr(err, "failed to add a member from chunk in state")
			}
		}

		for i := range ev.Presences {
			if err := s.Cabinet.PresenceSet(ev.GuildID, &ev.Presences[i], false); err != nil {
				s.stateErr(err, "failed to add a presence from chunk in state")
			}
		}

	case *gateway.GuildRoleCreateEvent:
		if err := s.Cabinet.RoleSet(ev.GuildID, &ev.Role, false); err != nil {
			s.stateErr(err, "failed to add a role in state")
		}

	case *gateway.GuildRoleUpdateEvent:
		if err := s.Cabinet.RoleSet(ev.GuildID, &ev.Role, true); err != nil {
			s.stateErr(err, "failed to update a role in state")
		}

	case *gateway.GuildRoleDeleteEvent:
		if err := s.Cabinet.RoleRemove(ev.GuildID, ev.RoleID); err != nil {
			s.stateErr(err, "failed to remove a role in state")
		}

	case *gateway.GuildEmojisUpdateEvent:
		if err := s.Cabinet.EmojiSet(ev.GuildID, ev.Emojis, true); err != nil {
			s.stateErr(err, "failed to update emojis in state")
		}

	case *gateway.ChannelCreateEvent:
		if err := s.Cabinet.ChannelSet(&ev.Channel, false); err != nil {
			s.stateErr(err, "failed to create a channel in state")
		}

	case *gateway.ChannelUpdateEvent:
		if err := s.Cabinet.ChannelSet(&ev.Channel, true); err != nil {
			s.stateErr(err, "failed to update a channel in state")
		}

	case *gateway.ChannelDeleteEvent:
		if err := s.Cabinet.ChannelRemove(&ev.Channel); err != nil {
			s.stateErr(err, "failed to remove a channel in state")
		}

		// The threads of a channel are deleted with it.
		s.removeThreads(ev.GuildID, func(thread discord.Channel) bool {
			return thread.ParentID == ev.ID
		})

	case *gateway.ChannelPinsUpdateEvent:
		// not tracked.

	case *gateway.ThreadListSyncEvent:
		// The event holds every active thread of the channels it names, or of
		// the guild if it names none, so the others are no longer active.
		s.removeThreads(ev.GuildID, func(thread discord.Channel) bool {
			synced := ev.ChannelIDs == nil || slices.Contains(ev.ChannelIDs, thread.ParentID)
			return synced && !slices.ContainsFunc(ev.Threads, func(t discord.Channel) bool {
				return t.ID == thread.ID
			})
		})

		for _, thread := range ev.Threads {
			// The event holds the current user's member of each thread in it
			// that they have joined, apart from the threads.
			thread.ThreadMember = nil
			if i := slices.IndexFunc(ev.Members, func(m discord.ThreadMember) bool { return m.ID == thread.ID }); i >= 0 {
				thread.ThreadMember = &ev.Members[i]
			}

			if err := s.Cabinet.ChannelSet(&thread, true); err != nil {
				s.stateErr(err, "failed to set a thread in state sync")
			}
		}

	case *gateway.ThreadCreateEvent:
		if err := s.Cabinet.ChannelSet(&ev.Channel, false); err != nil {
			s.stateErr(err, "failed to create a thread in state")
		}

	case *gateway.ThreadUpdateEvent:
		// Only active threads are kept.
		if ev.ThreadMetadata != nil && ev.ThreadMetadata.Archived {
			if err := s.Cabinet.ChannelRemove(&ev.Channel); err != nil {
				s.stateErr(err, "failed to remove an archived thread in state")
			}

			break
		}

		// The update does not hold the current user's member of the thread.
		thread := ev.Channel
		if old, err := s.Cabinet.Channel(thread.ID); err == nil {
			thread.ThreadMember = old.ThreadMember
		}

		if err := s.Cabinet.ChannelSet(&thread, true); err != nil {
			s.stateErr(err, "failed to update a thread in state")
		}

	case *gateway.ThreadMemberUpdateEvent:
		s.setThreadMember(ev.ID, &ev.ThreadMember)

	case *gateway.ThreadMembersUpdateEvent:
		me, err := s.Cabinet.Me()
		if err != nil {
			break
		}

		if i := slices.IndexFunc(ev.AddedMembers, func(m discord.ThreadMember) bool { return m.UserID == me.ID }); i >= 0 {
			s.setThreadMember(ev.ID, &ev.AddedMembers[i])
		} else if slices.Contains(ev.RemovedMemberIDs, me.ID) {
			s.setThreadMember(ev.ID, nil)
		}

	case *gateway.ThreadDeleteEvent:
		if ch, err := s.Cabinet.Channel(ev.ID); err == nil {
			if err := s.Cabinet.ChannelRemove(ch); err != nil {
				s.stateErr(err, "failed to delete a thread in state")
			}
		}

	case *gateway.MessageCreateEvent:
		if ev.GuildID.IsValid() && ev.Member != nil {
			ev.Member.User = ev.Author
			if err := s.Cabinet.MemberStore.MemberSet(ev.GuildID, ev.Member, false); err != nil {
				s.stateErr(err, "failed to add a member in state from message create")
			}
		}
		if err := s.Cabinet.MessageSet(&ev.Message, false); err != nil {
			s.stateErr(err, "failed to add a message in state")
		}

	case *gateway.MessageUpdateEvent:
		if ev.GuildID.IsValid() && ev.Member != nil {
			ev.Member.User = ev.Author
			if err := s.Cabinet.MemberStore.MemberSet(ev.GuildID, ev.Member, false); err != nil {
				s.stateErr(err, "failed to add a member in state from message create")
			}
		}
		if err := s.Cabinet.MessageSet(&ev.Message, true); err != nil {
			s.stateErr(err, "failed to update a message in state")
		}

	case *gateway.MessageDeleteEvent:
		if err := s.Cabinet.MessageRemove(ev.ChannelID, ev.ID); err != nil {
			s.stateErr(err, "failed to delete a message in state")
		}

	case *gateway.MessageDeleteBulkEvent:
		for _, id := range ev.IDs {
			if err := s.Cabinet.MessageRemove(ev.ChannelID, id); err != nil {
				s.stateErr(err, "failed to delete bulk messages in state")
			}
		}

	case *gateway.MessageReactionAddEvent:
		me, _ := s.Cabinet.Me()
		s.addReaction(ev.ChannelID, ev.MessageID, ev.Emoji, 1, me != nil && ev.UserID == me.ID)

	case *gateway.MessageReactionAddManyEvent:
		me, _ := s.Cabinet.Me()
		for _, reaction := range ev.Reactions {
			s.addReaction(ev.ChannelID, ev.MessageID, reaction.Emoji, len(reaction.Users), me != nil && slices.Contains(reaction.Users, me.ID))
		}

	case *gateway.MessageReactionRemoveEvent:
		s.editMessage(ev.ChannelID, ev.MessageID, func(m *discord.Message) bool {
			var i = findReaction(m.Reactions, ev.Emoji)
			if i < 0 {
				return false
			}

			r := &m.Reactions[i]
			// If the count is 0:
			if (r.Count - 1) < 1 {
				m.Reactions = slices.Delete(m.Reactions, i, i+1)
			} else {
				r.Count--
				if r.Me { // If reaction removal is the user's
					u, err := s.Cabinet.Me()
					if err == nil && ev.UserID == u.ID {
						r.Me = false
					}
				}
			}

			return true
		})

	case *gateway.MessageReactionRemoveAllEvent:
		s.editMessage(ev.ChannelID, ev.MessageID, func(m *discord.Message) bool {
			m.Reactions = nil
			return true
		})

	case *gateway.MessageReactionRemoveEmojiEvent:
		s.editMessage(ev.ChannelID, ev.MessageID, func(m *discord.Message) bool {
			var i = findReaction(m.Reactions, ev.Emoji)
			if i < 0 {
				return false
			}
			m.Reactions = slices.Delete(m.Reactions, i, i+1)
			return true
		})

	case *gateway.PresenceUpdateEvent:
		if err := s.Cabinet.PresenceSet(ev.GuildID, &ev.Presence, true); err != nil {
			s.stateErr(err, "failed to update presence in state")
		}

	case *gateway.PresencesReplaceEvent:
		for _, p := range *ev {
			if err := s.Cabinet.PresenceSet(p.GuildID, &p.Presence, true); err != nil {
				s.stateErr(err, "failed to update presence in state")
			}
		}

	case *gateway.UserSettingsUpdateEvent:
		s.readyMu.Lock()
		s.ready.UserSettings = &ev.UserSettings
		s.readyMu.Unlock()

	case *gateway.UserUpdateEvent:
		if err := s.Cabinet.MyselfSet(ev.User, true); err != nil {
			s.stateErr(err, "failed to update myself from USER_UPDATE")
		}

	case *gateway.VoiceStateUpdateEvent:
		vs := &ev.VoiceState
		if !vs.ChannelID.IsValid() {
			if err := s.Cabinet.VoiceStateRemove(vs.GuildID, vs.UserID); err != nil {
				s.stateErr(err, "failed to remove voice state from state")
			}
		} else {
			if err := s.Cabinet.VoiceStateSet(vs.GuildID, vs, true); err != nil {
				s.stateErr(err, "failed to update voice state in state")
			}
		}
	}
}

func (s *State) storeMerged(guilds []gateway.GuildCreateEvent, mergedMembers [][]gateway.SupplementalMember, mergedPresences gateway.MergedPresences) {
	for i := range guilds {
		if i < len(mergedMembers) {
			members := gateway.ConvertSupplementalMembers(mergedMembers[i])
			for j := range members {
				members[j].User = s.cachedUser(members[j].User.ID)
				if err := s.Cabinet.MemberSet(guilds[i].ID, &members[j], false); err != nil {
					s.stateErr(err, "failed to set merged member")
				}
			}
		}

		if i < len(mergedPresences.Guilds) {
			presences := gateway.ConvertSupplementalPresences(mergedPresences.Guilds[i])
			for j := range presences {
				presences[j].User = s.cachedUser(presences[j].User.ID)
				if err := s.Cabinet.PresenceSet(guilds[i].ID, &presences[j], false); err != nil {
					s.stateErr(err, "failed to set merged presence")
				}
			}
		}
	}

	friends := gateway.ConvertSupplementalPresences(mergedPresences.Friends)
	for i := range friends {
		friends[i].User = s.cachedUser(friends[i].User.ID)
		if err := s.Cabinet.PresenceSet(0, &friends[i], false); err != nil {
			s.stateErr(err, "failed to set merged friend presence")
		}
	}
}

func (s *State) cachedUser(id discord.UserID) discord.User {
	if user, err := s.Cabinet.User(id); err == nil {
		return *user
	}
	return discord.User{ID: id}
}

// setThreadMember sets the current user's member of the thread, or nil once
// they left it.
func (s *State) setThreadMember(threadID discord.ChannelID, member *discord.ThreadMember) {
	thread, err := s.Cabinet.Channel(threadID)
	if err != nil {
		return
	}

	thread.ThreadMember = member
	if err := s.Cabinet.ChannelSet(thread, true); err != nil {
		s.stateErr(err, "failed to set the thread member in state")
	}
}

// removeThreads removes the threads of the guild that remove reports true for.
func (s *State) removeThreads(guildID discord.GuildID, remove func(thread discord.Channel) bool) {
	channels, _ := s.Cabinet.Channels(guildID)
	for i := range channels {
		if !channels[i].Type.IsThread() || !remove(channels[i]) {
			continue
		}

		if err := s.Cabinet.ChannelRemove(&channels[i]); err != nil {
			s.stateErr(err, "failed to remove a thread in state")
		}
	}
}

func (s *State) stateErr(err error, wrap string) {
	s.StateLog(fmt.Errorf("%s: %w", wrap, err))
}

func (s *State) batchLog(errors []error) {
	for _, err := range errors {
		s.StateLog(err)
	}
}

// Helper functions

func (s *State) addReaction(channelID discord.ChannelID, messageID discord.MessageID, emoji discord.Emoji, count int, me bool) {
	s.editMessage(channelID, messageID, func(m *discord.Message) bool {
		m.Reactions = slices.Clone(m.Reactions)
		i := findReaction(m.Reactions, emoji)
		if i < 0 {
			m.Reactions = append(m.Reactions, discord.Reaction{Count: count, Me: me, Emoji: emoji})
		} else {
			m.Reactions[i].Count += count
			m.Reactions[i].Me = m.Reactions[i].Me || me
		}
		return true
	})
}

func (s *State) editMessage(ch discord.ChannelID, msg discord.MessageID, fn func(m *discord.Message) bool) {
	m, err := s.Cabinet.Message(ch, msg)
	if err != nil {
		return
	}

	// Copy the messages.
	cpy := *m
	m = &cpy

	if !fn(m) {
		return
	}

	if err := s.Cabinet.MessageSet(m, true); err != nil {
		s.stateErr(err, "failed to save message in reaction add")
	}
}

func findReaction(rs []discord.Reaction, emoji discord.Emoji) int {
	for i := range rs {
		if rs[i].Emoji.ID == emoji.ID && rs[i].Emoji.Name == emoji.Name {
			return i
		}
	}
	return -1
}

func storeGuildCreate(cab *store.Cabinet, guild *gateway.GuildCreateEvent) []error {
	if guild.Unavailable {
		return nil
	}

	stack, errs := newErrorStack()

	if err := cab.GuildSet(&guild.Guild, false); err != nil {
		errs(err, "failed to set guild in Ready")
	}

	// Handle guild emojis
	if len(guild.Emojis) > 0 {
		if err := cab.EmojiSet(guild.ID, guild.Emojis, false); err != nil {
			errs(err, "failed to set guild emojis")
		}
	}

	// Handle guild member
	for i := range guild.Members {
		if err := cab.MemberSet(guild.ID, &guild.Members[i], false); err != nil {
			errs(err, "failed to set guild member in Ready")
		}
	}

	// Handle guild channels
	for _, ch := range guild.Channels {
		// I HATE Discord.
		ch.GuildID = guild.ID

		if err := cab.ChannelSet(&ch, false); err != nil {
			errs(err, "failed to set guild channel in Ready")
		}
	}

	// Handle threads.
	for _, ch := range guild.Threads {
		ch.GuildID = guild.ID

		if err := cab.ChannelSet(&ch, false); err != nil {
			errs(err, "failed to set guild thread in Ready")
		}
	}

	// Handle guild presences
	for _, p := range guild.Presences {
		p.GuildID = guild.ID

		if err := cab.PresenceSet(guild.ID, &p, false); err != nil {
			errs(err, "failed to set guild presence in Ready")
		}
	}

	// Handle guild voice states
	for _, v := range guild.VoiceStates {
		v.GuildID = guild.ID

		if err := cab.VoiceStateSet(guild.ID, &v, false); err != nil {
			errs(err, "failed to set guild voice state in Ready")
		}
	}

	// Handle guild roles
	for _, r := range guild.Roles {

		if err := cab.RoleSet(guild.ID, &r, false); err != nil {
			errs(err, "failed to set role in Ready")
		}
	}

	return *stack
}

func newErrorStack() (*[]error, func(error, string)) {
	var errs = new([]error)
	return errs, func(err error, wrap string) {
		*errs = append(*errs, fmt.Errorf("%s: %w", wrap, err))
	}
}
