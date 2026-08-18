package defaultstore

import (
	"sync"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/state/store"
)

type Member struct {
	guilds *atomicMap[discord.GuildID, *guildMembers]
}

type guildMembers struct {
	mut     sync.RWMutex
	members map[discord.UserID]discord.Member
}

var _ store.MemberStore = (*Member)(nil)

func NewMember() *Member {
	return &Member{
		guilds: newAtomicMap[discord.GuildID](func() *guildMembers {
			return &guildMembers{
				members: make(map[discord.UserID]discord.Member, 1),
			}
		}),
	}
}

func (s *Member) Reset() error {
	s.guilds.Reset()
	return nil
}

func (s *Member) Member(guildID discord.GuildID, userID discord.UserID) (*discord.Member, error) {
	gm, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	gm.mut.RLock()
	defer gm.mut.RUnlock()

	m, ok := gm.members[userID]
	if ok {
		return &m, nil
	}

	return nil, store.ErrNotFound
}

func (s *Member) Members(guildID discord.GuildID) ([]discord.Member, error) {
	gm, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	gm.mut.RLock()
	defer gm.mut.RUnlock()

	var members = make([]discord.Member, 0, len(gm.members))
	for _, m := range gm.members {
		members = append(members, m)
	}

	return members, nil
}

func (s *Member) MemberSet(guildID discord.GuildID, m *discord.Member, update bool) error {
	gm, _ := s.guilds.LoadOrStore(guildID)

	gm.mut.Lock()
	if _, ok := gm.members[m.User.ID]; !ok || update {
		gm.members[m.User.ID] = *m
	}
	gm.mut.Unlock()

	return nil
}

func (s *Member) MemberRemove(guildID discord.GuildID, userID discord.UserID) error {
	gm, ok := s.guilds.Load(guildID)
	if !ok {
		return nil
	}

	gm.mut.Lock()
	delete(gm.members, userID)
	gm.mut.Unlock()

	return nil
}
