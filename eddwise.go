package eddwise

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/ugorji/go/codec"
)

//go:embed eddclient.js
var eddclientJS []byte

func ClientJS() []byte {
	var ret = make([]byte, len(eddclientJS))
	copy(ret, eddclientJS)
	return ret
}

func ErrMissingServerHandler(chName, eventName string) error {
	if len(eventName) == 0 {
		return fmt.Errorf("empty event name")
	}
	return fmt.Errorf("handler for event '%s' on channel '%s' was not expected", eventName, chName)
}

type ClientContext interface {
	Has(string) bool
	Get(string) interface{}
	Set(string, interface{})
	setRawAuth(*Auth)
	GetRawAuth() *Auth
	setState(interface{})
	GetState() interface{}
}

type ClientContextMap struct {
	auth  *Auth
	state interface{}
	m     map[string]interface{}
}

func (cc *ClientContextMap) Has(key string) bool {
	_, ok := cc.m[key]
	return ok
}

func (cc *ClientContextMap) Get(key string) interface{} {
	return cc.m[key]
}

func (cc *ClientContextMap) Set(key string, value interface{}) {
	cc.m[key] = value
}

func (cc *ClientContextMap) setRawAuth(auth *Auth) {
	cc.auth = auth
}

func (cc *ClientContextMap) GetRawAuth() *Auth {
	if cc.auth == nil {
		log.Println("an empty auth was requested")
	}
	return cc.auth
}

func (cc *ClientContextMap) setState(state interface{}) {
	cc.state = state
}

func (cc *ClientContextMap) GetState() interface{} {
	return cc.state
}

type Client interface {
	ClientContext
	GetId() uint64
	Send(channel string, event Event) error
	SendJSON(interface{}) error
	Close() error
	Closed() bool
}

type ClientSocket struct {
	ClientContextMap
	id      uint64
	Server  *ServerSocket
	Conn    *websocket.Conn
	WriteMx sync.Mutex
	closed  bool
}

func (c *ClientSocket) GetId() uint64 {
	return c.id
}

func (c *ClientSocket) Send(channel string, event Event) error {
	if ecf, ok := event.(EventCheckSendFields); ok {
		if err := ecf.CheckSendFields(); err != nil {
			return err
		}
	}
	if c.closed {
		return errors.New("writing to closed client")
	}
	var evt = &EventMessageToSend{
		Channel: channel,
		Name:    event.ProtocolAlias(),
		Body:    event,
	}
	//var err error
	//evt.Body, err = c.Server.Codec().Encode(event)
	//if err != nil {
	//	return err
	//}

	m, err := c.Server.Codec().Encode(evt)
	if err != nil {
		return fmt.Errorf("cannot encode message: %w", err)
	}
	c.WriteMx.Lock()
	defer c.WriteMx.Unlock()
	var mt int
	switch c.Server.codec.handle.(type) {
	case *codec.JsonHandle:
		mt = websocket.TextMessage
	case *codec.MsgpackHandle:
		mt = websocket.BinaryMessage
	}
	if err := c.Conn.WriteMessage(mt, m); err != nil {
		return err
	}
	return nil
}

func (c *ClientSocket) SendJSON(v interface{}) error {
	c.WriteMx.Lock()
	defer c.WriteMx.Unlock()
	return c.Conn.WriteJSON(v)
}

func (c *ClientSocket) Closed() bool {
	return c.closed
}

func (c *ClientSocket) Close() error {
	c.closed = true
	return c.Conn.Close()
}

type CodecSerializer struct {
	handle codec.Handle
}

func NewCodecSerializer(handle codec.Handle) *CodecSerializer {
	return &CodecSerializer{handle}
}

func (cs *CodecSerializer) Encode(v interface{}) ([]byte, error) {
	var buf = make([]byte, 128)
	var err = codec.NewEncoderBytes(&buf, cs.handle).Encode(v)
	return buf, err
}

func (cs *CodecSerializer) Decode(data []byte, v interface{}) error {
	var err = codec.NewDecoderBytes(data, cs.handle).Decode(v)
	return err
}

type Server interface {
	AddClient(Client)
	GetClients(...uint64) []Client
	GetClient(uint64) Client
	RemoveClient(Client)
	Codec() *CodecSerializer
}

var _ Server = (*ServerSocket)(nil)

type ServerSocket struct {
	Conn               net.Conn
	registeredStatic   map[string]string
	RegisteredChannels map[string]ImplChannel
	codec              *CodecSerializer
	ClientAutoInc      uint64
	Clients            map[uint64]Client
	ClientsMx          sync.RWMutex
	App                *fiber.App
}

func NewServer() *ServerSocket {
	return NewServerWithCustomCodec(&codec.JsonHandle{})
}

func NewServerWithCustomCodec(codec codec.Handle) *ServerSocket {
	return &ServerSocket{
		codec:              NewCodecSerializer(codec),
		registeredStatic:   make(map[string]string),
		RegisteredChannels: make(map[string]ImplChannel),
		Clients:            make(map[uint64]Client),
	}
}

func (s *ServerSocket) AddClient(c Client) {
	s.ClientsMx.Lock()
	defer s.ClientsMx.Unlock()
	s.Clients[c.GetId()] = c
}

func (s *ServerSocket) GetClient(id uint64) Client {
	s.ClientsMx.RLock()
	defer s.ClientsMx.RUnlock()
	return s.Clients[id]
}

func (s *ServerSocket) RemoveClient(c Client) {
	s.ClientsMx.Lock()
	defer s.ClientsMx.Unlock()
	delete(s.Clients, c.GetId())
}

func (s *ServerSocket) RegisterStatic(path, dir string) {
	s.registeredStatic[path] = dir
}

func (s *ServerSocket) CustomFiberApp(app *fiber.App) {
	s.App = app
}

func (s *ServerSocket) initWS(wsPath string) {
	if s.App == nil {
		s.App = fiber.New()
	}

	for path, dir := range s.registeredStatic {
		s.App.Static(path, dir)
	}

	s.App.Use(wsPath, func(c *fiber.Ctx) error {
		if bytes.HasSuffix(c.Request().URI().Path(), []byte("/edd.js")) {
			c.Response().Header.Add("content-type", "application/javascript")
			return c.Send(eddclientJS)
		}
		log.Println("mw", c.Request().URI(), c.IP())
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	s.App.Get(wsPath, websocket.New(func(c *websocket.Conn) {
		defer func() { _ = c.Close() }()

		log.Println("new client is connecting", c.RemoteAddr().String())
		var client = &ClientSocket{
			ClientContextMap: ClientContextMap{auth: nil, m: map[string]interface{}{}},
			Conn:             c,
			Server:           s,
			id:               atomic.AddUint64(&s.ClientAutoInc, 1),
		}
		var ctx = NewDefaultContext(context.Background(), s, client)

		if err := s.CheckAuth(ctx, client); err != nil {
			var ee = EventMessageToSend{
				Channel: "errors",
				Name:    "error",
				Body:    fmt.Sprintf("auth error: %s", err),
			}
			if err := client.SendJSON(ee); err != nil {
				log.Println("unable to write err json on auth: ", err)
			}
			return
		}

		defer func() {
			_ = s.RevokeAuth(ctx, client)
		}()

		//check if it is able to connect to all channels
		for _, ch := range s.RegisteredChannels {
			if connRecv, ok := ch.(ImplChannelConnected); ok {
				if err := connRecv.Connected(client); err != nil {
					var ee = EventMessage{
						Channel: "errors",
						Name:    "error",
					}
					ee.Body, _ = json.Marshal(fmt.Sprintf("error while connecting on %s: %s", ch.Name(), err))
					if err := client.SendJSON(ee); err != nil {
						log.Println("unable to write err json on connected: ", err)
					}
					return
				}
			}
		}

		s.AddClient(client)
		defer func() {
			s.RemoveClient(client)
		}()

		defer func() {
			for _, ch := range s.RegisteredChannels {
				if connRecv, ok := ch.(ImplChannelDisconnected); ok {
					_ = connRecv.Disconnected(client)
				}
			}
		}()

		//Auto broadcast Join
		for _, ch := range s.RegisteredChannels {
			if _, ok := ch.(ImplConnManager); !ok {
				if chUser, ok := ch.(ImplChannelWithUserJoin); ok {
					_ = chUser.onJoin(ch, client, true)
				}
			}
		}

		//Auto broadcast Left
		defer func() {
			for _, ch := range s.RegisteredChannels {
				if _, ok := ch.(ImplConnManager); !ok {
					if chUser, ok := ch.(ImplChannelWithUserLeft); ok {
						_ = chUser.onLeft(ch, client)
					}
				}
			}
		}()

		var (
			//mt  int
			msg []byte
			err error
		)
		for {
			if _, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				_ = client.Close()
				break
			}

			if err := s.ProcessEvent(ctx, msg); err != nil {
				var ee = EventMessage{
					Channel: "errors",
					Name:    "error",
				}
				ee.Body, _ = json.Marshal(fmt.Sprintf("error while processing event: %s", err))
				if err := client.SendJSON(ee); err != nil {
					log.Println("unable to write err json: ", err)
				}
			}

		}

	}, websocket.Config{
		EnableCompression: true,
	}))
}

func (s *ServerSocket) StartWS(wsPath string, port int) error {
	s.initWS(wsPath)
	return s.App.Listen(fmt.Sprintf(":%d", port))
}

func (s *ServerSocket) StartWSS(wsPath string, port int, certFile, keyFile string) error {
	s.initWS(wsPath)
	return s.App.ListenTLS(fmt.Sprintf(":%d", port), certFile, keyFile)
}

func (s *ServerSocket) Close() error {
	return s.App.Shutdown()
}

func (s *ServerSocket) Register(ch ImplChannel) error {
	if _, ok := s.RegisteredChannels[ch.Alias()]; ok {
		return fmt.Errorf("channel '%s' is already registered (alias: '%s')", ch.Name(), ch.Alias())
	}
	if err := ch.Bind(s); err != nil {
		return err
	}

	if chAuth, ok := ch.(ImplConnManager); ok {
		chAuth.connManagerInit()
	}
	s.RegisteredChannels[ch.Alias()] = ch
	return ch.SetReceiver(ch)
}

func (s *ServerSocket) ProcessEvent(ctx Context, rawEvent []byte) error {
	var event = &EventMessage{}
	if err := s.Codec().Decode(rawEvent, event); err != nil {
		return err
	}
	if len(event.Channel) == 0 {
		return fmt.Errorf("empty channel")
	}
	ch, ok := s.RegisteredChannels[event.Channel]
	if !ok {
		return fmt.Errorf("unknown channel %s", event.Channel)
	}

	return ch.Route(ctx, event)
}

func (s *ServerSocket) GetClients(exclude ...uint64) []Client {
	s.ClientsMx.RLock()
	defer s.ClientsMx.RUnlock()
	var ret = make([]Client, 0, len(s.Clients))
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

func (s *ServerSocket) Codec() *CodecSerializer {
	return s.codec
}

func Broadcast(channel string, event Event, clients []Client) error {
	if ecf, ok := event.(EventCheckSendFields); ok {
		if err := ecf.CheckSendFields(); err != nil {
			return err
		}
	}
	var errCh = make(chan error, 1)
	var errs []error
	var wgErr sync.WaitGroup
	wgErr.Add(1)
	go func() {
		for err := range errCh {
			errs = append(errs, err)
		}
		wgErr.Done()
	}()
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c Client) {
			if err := c.Send(channel, event); err != nil {
				errCh <- err
			}
			wg.Done()
		}(c)
	}
	wg.Wait()
	close(errCh)
	wgErr.Wait()
	if len(errs) > 0 {
		var errmsg = bytes.NewBuffer(nil)
		_, _ = fmt.Fprintf(errmsg, "%d error(s) occurs while broadcasting:\n", len(errs))
		for _, err := range errs {
			_, _ = fmt.Fprintf(errmsg, "\t%s\n", err)
		}
		return errors.New(errmsg.String())
	}
	return nil
}

type Context interface {
	context.Context
	GetServer() Server
	GetClient() Client
}

type DefaultContext struct {
	context.Context
	server Server
	client Client
}

func NewDefaultContext(ctx context.Context, server Server, client Client) *DefaultContext {
	return &DefaultContext{
		Context: ctx,
		server:  server,
		client:  client,
	}
}
func NewDefaultContextFromBackground(server Server, client Client) *DefaultContext {
	return &DefaultContext{
		Context: context.Background(),
		server:  server,
		client:  client,
	}
}

func (ctx *DefaultContext) GetServer() Server {
	return ctx.server
}

func (ctx *DefaultContext) GetClient() Client {
	return ctx.client
}

type EventMessage struct {
	Channel string    `json:"channel"`
	Name    string    `json:"name"`
	Body    codec.Raw `json:"body"`
}

type EventHandler func(Context, *EventMessage) error

type EventMessageToSend struct {
	Channel string      `json:"channel"`
	Name    string      `json:"name"`
	Body    interface{} `json:"body"`
}

type ImplChannel interface {
	Name() string
	Alias() string
	Bind(Server) error
	Route(Context, *EventMessage) error
	GetServer() Server
	SetReceiver(ImplChannel) error
}

type ImplChannelConnected interface {
	Connected(Client) error
}

type ImplChannelDisconnected interface {
	Disconnected(Client) error
}

type TestableChannel interface {
	ImplChannel
	Given(ImplChannel)
	Expect(TestableChannel)
}

type Event interface {
	GetEventName() string
	ProtocolAlias() string
}

type EventCheckReceivedFields interface {
	Event
	CheckReceivedFields() error
}
type EventCheckSendFields interface {
	Event
	CheckSendFields() error
}
