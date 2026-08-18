package defaultstore

import (
	"sync"

	"github.com/ayn2op/arikawa/v3/discord"
	"github.com/ayn2op/arikawa/v3/state/store"
)

type Role struct {
	guilds *atomicMap[discord.GuildID, *roles]
}

var _ store.RoleStore = (*Role)(nil)

type roles struct {
	mut   sync.RWMutex
	roles map[discord.RoleID]discord.Role
}

func NewRole() *Role {
	return &Role{
		guilds: newAtomicMap[discord.GuildID](func() *roles {
			return &roles{
				roles: make(map[discord.RoleID]discord.Role, 1),
			}
		}),
	}
}

func (s *Role) Reset() error {
	s.guilds.Reset()
	return nil
}

func (s *Role) Role(guildID discord.GuildID, roleID discord.RoleID) (*discord.Role, error) {
	rs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	rs.mut.RLock()
	defer rs.mut.RUnlock()

	r, ok := rs.roles[roleID]
	if ok {
		return &r, nil
	}

	return nil, store.ErrNotFound
}

func (s *Role) Roles(guildID discord.GuildID) ([]discord.Role, error) {
	rs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil, store.ErrNotFound
	}

	rs.mut.RLock()
	defer rs.mut.RUnlock()

	var roles = make([]discord.Role, 0, len(rs.roles))
	for _, role := range rs.roles {
		roles = append(roles, role)
	}

	return roles, nil
}

func (s *Role) RoleSet(guildID discord.GuildID, role *discord.Role, update bool) error {
	rs, _ := s.guilds.LoadOrStore(guildID)

	rs.mut.Lock()
	if _, ok := rs.roles[role.ID]; !ok || update {
		rs.roles[role.ID] = *role
	}
	rs.mut.Unlock()

	return nil
}

func (s *Role) RoleRemove(guildID discord.GuildID, roleID discord.RoleID) error {
	rs, ok := s.guilds.Load(guildID)
	if !ok {
		return nil
	}

	rs.mut.Lock()
	delete(rs.roles, roleID)
	rs.mut.Unlock()

	return nil
}
