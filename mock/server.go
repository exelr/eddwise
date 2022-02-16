package mock

import (
	"sync"

	"github.com/exelr/eddwise"
)

var _ eddwise.Server = (*ServerMock)(nil)

type ServerMock struct {
	Clients   map[uint64]eddwise.Client
	ClientsMx sync.RWMutex
}

func NewServer() *ServerMock {
	return &ServerMock{
		Clients: make(map[uint64]eddwise.Client),
	}
}

func (s *ServerMock) AddClient(c eddwise.Client) {
	s.ClientsMx.Lock()
	defer s.ClientsMx.Unlock()
	s.Clients[c.GetId()] = c
}

func (s *ServerMock) GetClient(id uint64) eddwise.Client {
	s.ClientsMx.RLock()
	defer s.ClientsMx.RUnlock()
	return s.Clients[id]
}

func (s *ServerMock) GetClients(exclude ...uint64) []eddwise.Client {
	s.ClientsMx.RLock()
	defer s.ClientsMx.RUnlock()
	var ret = make([]eddwise.Client, 0, len(s.Clients))
for1:
	for _, c := range s.Clients {
		for _, e := range exclude {
			if e == c.GetId() {
				continue for1
			}
		}
		ret = append(ret, c)
	}
	return ret
}

func (s *ServerMock) RemoveClient(c eddwise.Client) {
	s.ClientsMx.Lock()
	defer s.ClientsMx.Unlock()
	delete(s.Clients, c.GetId())
}

func (s *ServerMock) Codec() *eddwise.CodecSerializer {
	return nil
}
