package eddwise

import (
	"fmt"
	"sync"

	"golang.org/x/exp/slices"
)

type AuthChallenge struct {
	Methods []string `json:"methods"`
}

func (*AuthChallenge) GetEventName() string {
	return "edd:auth:challenge"
}

func (*AuthChallenge) ProtocolAlias() string {
	return "edd:auth:challenge"
}

type AuthPass struct {
	Id string `json:"id"`
}

func (*AuthPass) GetEventName() string {
	return "edd:auth:pass"
}

func (*AuthPass) ProtocolAlias() string {
	return "edd:auth:pass"
}

type UserState[T any] struct {
	sync.RWMutex
	State T
}

type Auth struct {
	Id   string      `json:"id"`
	Data interface{} `json:"data"`
}

type StateManager[T any] struct{}

func (sm *StateManager[T]) SetClientState(c Client, state T) {
	c.setState(&UserState[T]{
		State: state,
	})
}

func (sm *StateManager[T]) ClientState(c Client) *UserState[T] {
	return c.GetState().(*UserState[T])
}

type ImplConnManager interface {
	connManagerInit()
	setAuth(Client, *Auth) (firstConnection bool)
	removeAuth(uint64) (anyConnected bool)
	GetAuthorizedUserClients(...string) []Client
	GetAuthorizedClients(...uint64) []Client
	GetAuthorizedUserIds(...string) []string
}

type ConnManager struct {
	mx              sync.RWMutex
	clients         map[uint64]Client
	userConnections map[string]map[uint64]Client
}

func (cm *ConnManager) connManagerInit() {
	//cm.auths = map[uint64]*Auth{}
	cm.clients = map[uint64]Client{}
	cm.userConnections = map[string]map[uint64]Client{}
}

func (cm *ConnManager) setAuth(client Client, a *Auth) bool {
	cm.mx.Lock()
	defer cm.mx.Unlock()

	client.setRawAuth(a)

	cm.clients[client.GetId()] = client

	if _, ok := cm.userConnections[a.Id]; !ok {
		cm.userConnections[a.Id] = map[uint64]Client{}
	}
	cm.userConnections[a.Id][client.GetId()] = client

	return len(cm.userConnections[a.Id]) == 1
}

func (cm *ConnManager) removeAuth(clientId uint64) bool {
	cm.mx.Lock()
	defer cm.mx.Unlock()
	auth := cm.clients[clientId].GetRawAuth()
	delete(cm.clients, clientId)
	delete(cm.userConnections[auth.Id], clientId)
	if len(cm.userConnections[auth.Id]) == 0 {
		delete(cm.userConnections, auth.Id)
		return false
	}
	return true
}

func (cm *ConnManager) GetAuthorizedUserClients(exceptUserIds ...string) []Client {
	cm.mx.RLock()
	defer cm.mx.RUnlock()
	var ret = make([]Client, 0, len(cm.userConnections))
	for clientId, client := range cm.clients {
		if slices.Contains(exceptUserIds, cm.clients[clientId].GetRawAuth().Id) {
			continue
		}
		ret = append(ret, client)
	}
	return ret
}

func (cm *ConnManager) GetAuthorizedClients(exceptClientIds ...uint64) []Client {
	cm.mx.RLock()
	defer cm.mx.RUnlock()
	var ret = make([]Client, 0, len(cm.userConnections))
	for clientId, client := range cm.clients {
		if slices.Contains(exceptClientIds, clientId) {
			continue
		}
		ret = append(ret, client)
	}
	return ret
}

func (cm *ConnManager) IsUserConnected(id string) bool {
	cm.mx.RLock()
	defer cm.mx.RUnlock()
	_, ok := cm.userConnections[id]
	return ok
}

func (cm *ConnManager) GetAuthorizedUserIds(exceptIds ...string) []string {
	cm.mx.RLock()
	defer cm.mx.RUnlock()
	var ret []string
	for id := range cm.userConnections {
		if slices.Contains(exceptIds, id) {
			continue
		}
		ret = append(ret, id)
	}
	return ret
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (*BasicAuth) GetEventName() string {
	return "edd:auth:basic"
}

func (*BasicAuth) ProtocolAlias() string {
	return "edd:auth:basic"
}

func ChannelAuthMethods(ch ImplChannel) []string {
	var methods []string
	if _, ok := ch.(ImplChannelBasicAuth); ok {
		methods = append(methods, "edd:auth:basic")
	}
	return methods
}

type ImplChannelBasicAuth interface {
	OnBasicAuth(Context, *BasicAuth) (*Auth, error)
}

func (s *ServerSocket) CheckAuth(ctx Context, client *ClientSocket) error {

	for _, ch := range s.RegisteredChannels {
		if authMethods := ChannelAuthMethods(ch); len(authMethods) > 0 {
			if err := client.Send(ch.Alias(), &AuthChallenge{Methods: authMethods}); err != nil {
				return fmt.Errorf("unable to send auth challenge to %d: %w", client.id, err)
			}
			var msg []byte
			var err error
			if _, msg, err = client.Conn.ReadMessage(); err != nil {
				return fmt.Errorf("auth read: %w", err)

			}
			if err := s.ProcessEventAuth(ctx, ch, msg); err != nil {
				return err
			}
			if err := client.Send(ch.Alias(), &AuthPass{Id: ctx.GetClient().GetRawAuth().Id}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ServerSocket) RevokeAuth(ctx Context, client *ClientSocket) error {
	for _, ch := range s.RegisteredChannels {
		if chAuth, ok := ch.(ImplConnManager); ok {
			if !chAuth.removeAuth(client.id) {
				if chLeft, ok := ch.(ImplChannelWithUserLeft); ok {
					if err := chLeft.onLeft(ch, ctx.GetClient()); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (s *ServerSocket) ProcessEventAuth(ctx Context, chAuth ImplChannel, rawEvent []byte) error {
	var event = &EventMessage{}
	if err := s.Codec().Decode(rawEvent, event); err != nil {
		return fmt.Errorf("decoding error: %w", err)
	}
	if len(event.Channel) == 0 {
		return fmt.Errorf("empty channel")
	}
	ch, ok := s.RegisteredChannels[event.Channel]
	if !ok {
		return fmt.Errorf("unknown channel %s", event.Channel)
	}
	if ch != chAuth {
		return fmt.Errorf("ch auth mismatch")
	}
	switch event.Name {
	case "edd:auth:basic":
		chBasic, ok := ch.(ImplChannelBasicAuth)
		if !ok {
			return fmt.Errorf("basic auth not supported")
		}
		var ba = &BasicAuth{}
		if err := s.codec.Decode(event.Body, ba); err != nil {
			return err
		}
		auth, err := chBasic.OnBasicAuth(ctx, ba)
		if err != nil {
			return err
		}
		ctx.GetClient().setRawAuth(auth)
	default:
		return fmt.Errorf("unknown auth method")
	}

	if chAuthMan, ok := ch.(ImplConnManager); ok {
		var first = chAuthMan.setAuth(ctx.GetClient(), ctx.GetClient().GetRawAuth())
		if chJoin, ok := ch.(ImplChannelWithUserJoin); ok {
			return chJoin.onJoin(ch, ctx.GetClient(), first)
		}
	}

	return nil
}
