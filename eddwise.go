package eddwise

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

//go:embed eddclient.js
var eddclientJS []byte

func ErrMissingServerHandler(chName, eventName string) error {
	if len(eventName) == 0 {
		return fmt.Errorf("empty event name")
	}
	return fmt.Errorf("handler for event '%s' on channel '%s' was not expected", eventName, chName)
}

type Client struct {
	id      uint64
	Server  *Server
	Conn    *websocket.Conn
	WriteMx sync.Mutex
	Closed  bool
}

func (c *Client) GetId() uint64 {
	return c.id
}

func (c *Client) Send(channel, name string, body interface{}) error {
	if c.Closed {
		return errors.New("writing to closed client")
	}
	var evt = &Event{
		Channel: channel,
		Name:    name,
	}
	var err error
	evt.Body, err = c.Server.Serializer.Serialize(body)
	if err != nil {
		return err
	}

	m, err := c.Server.Serializer.Serialize(evt)
	if err != nil {
		return err
	}
	c.WriteMx.Lock()
	defer c.WriteMx.Unlock()
	if err := c.Conn.WriteMessage(websocket.TextMessage, m); err != nil {
		return err
	}
	return nil
}

func (c *Client) SendJSON(v interface{}) error {
	c.WriteMx.Lock()
	defer c.WriteMx.Unlock()
	return c.Conn.WriteJSON(v)
}

type Server struct {
	Conn               net.Conn
	RegisteredChannels map[string]ImplChannel
	Serializer         Serializer
	ClientAutoInc      uint64
	Clients            map[uint64]*Client
	ClientsMx          sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		Serializer:         &JsonSerializer{},
		RegisteredChannels: make(map[string]ImplChannel),
		Clients:            make(map[uint64]*Client),
	}
}

func (s *Server) StartWS(wsPath string, port int) error {

	app := fiber.New()

	app.Use(wsPath, func(c *fiber.Ctx) error {
		if bytes.HasSuffix(c.Request().URI().Path(), []byte("/edd.js")) {
			c.Response().Header.Add("content-type", "application/javascript")
			return c.Send(eddclientJS)
		}
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get(wsPath, websocket.New(func(c *websocket.Conn) {

		var client = &Client{
			Conn:   c,
			Server: s,
			id:     atomic.AddUint64(&s.ClientAutoInc, 1),
		}

		//check if it is able to connect to all channels
		for _, ch := range s.RegisteredChannels {
			if connRecv, ok := ch.(ImplChannelConnected); ok {
				if err := connRecv.Connected(client); err != nil {
					var ee = Event{
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

		s.ClientsMx.Lock()
		s.Clients[client.id] = client
		s.ClientsMx.Unlock()

		defer func() {
			s.ClientsMx.Lock()
			delete(s.Clients, client.id)
			s.ClientsMx.Unlock()
		}()

		defer func() {
			for _, ch := range s.RegisteredChannels {
				if connRecv, ok := ch.(ImplChannelDisconnected); ok {
					_ = connRecv.Disconnected(client)
				}
			}
		}()
		defer func() { _ = c.Close() }()
		var ctx = NewDefaultContext(context.Background(), s, client)
		var (
			//mt  int
			msg []byte
			err error
		)
		for {
			if _, msg, err = c.ReadMessage(); err != nil {
				log.Println("read:", err)
				client.Closed = true
				break
			}

			if err := s.ProcessEvent(ctx, msg); err != nil {
				var ee = Event{
					Channel: "errors",
					Name:    "error",
				}
				ee.Body, _ = json.Marshal(fmt.Sprintf("error while processing event: %s", err))
				if err := client.SendJSON(ee); err != nil {
					log.Println("unable to write err json: ", err)
				}
			}

		}

	}))
	return app.Listen(fmt.Sprintf(":%d", port))
}

func (s *Server) Register(ch ImplChannel) error {
	if _, ok := s.RegisteredChannels[ch.Name()]; ok {
		return fmt.Errorf("channel '%s' is already registered", ch.Name())
	}
	if err := ch.Bind(s); err != nil {
		return err
	}
	s.RegisteredChannels[ch.Name()] = ch
	ch.SetReceiver(ch)
	return nil
}

func (s *Server) ProcessEvent(ctx Context, rawEvent []byte) error {
	var event = &Event{}
	if err := s.Serializer.Deserialize(rawEvent, event); err != nil {
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

func (s *Server) GetClients(exclude ...uint64) []*Client {
	s.ClientsMx.RLock()
	defer s.ClientsMx.RUnlock()
	var ret = make([]*Client, 0, len(s.Clients))
for1:
	for _, c := range s.Clients {
		for _, e := range exclude {
			if e == c.id {
				continue for1
			}
		}
		ret = append(ret, c)
	}
	return ret
}

func (s *Server) Broadcast(channel, name string, body interface{}, clients []*Client) error {
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
		go func(c *Client) {
			if err := c.Send(channel, name, body); err != nil {
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
	GetServer() *Server
	GetClient() *Client
}

type DefaultContext struct {
	context.Context
	server *Server
	client *Client
}

func NewDefaultContext(ctx context.Context, server *Server, client *Client) *DefaultContext {
	return &DefaultContext{
		Context: ctx,
		server:  server,
		client:  client,
	}
}

func (ctx *DefaultContext) GetServer() *Server {
	return ctx.server
}

func (ctx *DefaultContext) GetClient() *Client {
	return ctx.client
}

type EventHandler func(Context, *Event) error

type Event struct {
	Channel string          `json:"channel"`
	Name    string          `json:"name"`
	Body    json.RawMessage `json:"body"`
}

type ImplChannel interface {
	Name() string
	Bind(*Server) error
	Route(Context, *Event) error
	GetServer() *Server
	SetReceiver(interface{})
}

type ImplChannelConnected interface {
	Connected(*Client) error
}

type ImplChannelDisconnected interface {
	Disconnected(*Client) error
}
