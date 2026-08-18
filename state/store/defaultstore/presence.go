package defaultstore

import (
	"sync"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/state/store"
)

type Presence struct {
	guilds *atomicMap[discord.GuildID, *presences]
}

type presences struct {
	mut       sync.RWMutex
	presences map[discord.UserID]discord.Presence
}

var _ store.PresenceStore = (*Presence)(nil)

func NewPresence() *Presence {
	return &Presence{
		guilds: newAtomicMap[discord.GuildID](func() *presences {
			return &presences{
				presences: make(map[discord.UserID]discord.Presence, 1),
			}
		}),
	}
}

func (s *Presence) Reset() error {
	s.guilds.Reset()
	return nil
}

func (s *Presence) Presence(gID discord.GuildID, uID discord.UserID) (*discord.Presence, error) {
	ps, ok := s.guilds.Load(gID)
	if !ok {
		return nil, store.ErrNotFound
	}

	ps.mut.RLock()
	defer ps.mut.RUnlock()

	p, ok := ps.presences[uID]
	if ok {
		return &p, nil
	}

	return nil, store.ErrNotFound
}

func (s *Presence) Presences(guildID discord.GuildID) ([]discord.Presence, error) {
	ps, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	ps.mut.RLock()
	defer ps.mut.RUnlock()

	var presences = make([]discord.Presence, 0, len(ps.presences))
	for _, p := range ps.presences {
		presences = append(presences, p)
	}

	return presences, nil
}

func (s *Presence) PresenceSet(guildID discord.GuildID, p *discord.Presence, update bool) error {
	ps, _ := s.guilds.LoadOrStore(guildID)

	ps.mut.Lock()
	defer ps.mut.Unlock()

	// Shitty if check is better than a realloc every time.
	if ps.presences == nil {
		ps.presences = make(map[discord.UserID]discord.Presence, 1)
	}

	if _, ok := ps.presences[p.User.ID]; !ok || update {
		ps.presences[p.User.ID] = *p
	}

	return nil
}

func (s *Presence) PresenceRemove(guildID discord.GuildID, userID discord.UserID) error {
	ps, ok := s.guilds.Load(guildID)
	if !ok {
		return nil
	}

	ps.mut.Lock()
	delete(ps.presences, userID)
	ps.mut.Unlock()

	return nil
}
