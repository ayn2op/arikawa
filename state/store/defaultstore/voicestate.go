package defaultstore

import (
	"sync"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/state/store"
)

type VoiceState struct {
	guilds *atomicMap[discord.GuildID, *voiceStates]
}

var _ store.VoiceStateStore = (*VoiceState)(nil)

type voiceStates struct {
	mut         sync.RWMutex
	voiceStates map[discord.UserID]discord.VoiceState
}

func NewVoiceState() *VoiceState {
	return &VoiceState{
		guilds: newAtomicMap[discord.GuildID](func() *voiceStates {
			return &voiceStates{
				voiceStates: make(map[discord.UserID]discord.VoiceState, 1),
			}
		}),
	}
}

func (s *VoiceState) Reset() error {
	s.guilds.Reset()
	return nil
}

func (s *VoiceState) VoiceState(
	guildID discord.GuildID, userID discord.UserID) (*discord.VoiceState, error) {

	vs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	vs.mut.RLock()
	defer vs.mut.RUnlock()

	v, ok := vs.voiceStates[userID]
	if ok {
		return &v, nil
	}

	return nil, store.ErrNotFound
}

func (s *VoiceState) VoiceStates(guildID discord.GuildID) ([]discord.VoiceState, error) {
	vs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	vs.mut.RLock()
	defer vs.mut.RUnlock()

	var states = make([]discord.VoiceState, 0, len(vs.voiceStates))
	for _, state := range vs.voiceStates {
		states = append(states, state)
	}

	return states, nil
}

func (s *VoiceState) VoiceStateSet(
	guildID discord.GuildID, voiceState *discord.VoiceState, update bool) error {

	vs, _ := s.guilds.LoadOrStore(guildID)

	vs.mut.Lock()
	if _, ok := vs.voiceStates[voiceState.UserID]; !ok || update {
		vs.voiceStates[voiceState.UserID] = *voiceState
	}
	vs.mut.Unlock()

	return nil
}

func (s *VoiceState) VoiceStateRemove(guildID discord.GuildID, userID discord.UserID) error {
	vs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil
	}

	vs.mut.Lock()
	delete(vs.voiceStates, userID)
	vs.mut.Unlock()

	return nil
}
