package eddwise

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"sync"
	"testing"
	"time"
)

var _ ImplChannel = (*TestChannel)(nil)

type TestChannel struct {
	s            Server
	connected    bool
	disconnected bool
	mx           sync.RWMutex //required to avoid race detectors
}

func (ch *TestChannel) GetConnected() bool {
	ch.mx.RLock()
	defer ch.mx.RUnlock()
	return ch.connected
}

func (ch *TestChannel) GetDisconnected() bool {
	ch.mx.RLock()
	defer ch.mx.RUnlock()
	return ch.disconnected
}

func (ch *TestChannel) Connected(_ Client) error {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	ch.connected = true
	return nil
}

func (ch *TestChannel) Disconnected(_ Client) error {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	ch.disconnected = true
	return nil
}

func (ch *TestChannel) SetReceiver(recv ImplChannel) error {
	if _, ok := recv.(*TestChannel); !ok {
		return fmt.Errorf("unexpected channel type while SetReceiver")
	}
	return nil
}

func (ch *TestChannel) Bind(s Server) error {
	ch.s = s
	return nil
}

func (ch *TestChannel) GetServer() Server {
	return ch.s
}

func (ch *TestChannel) Name() string {
	return "test"
}
func (ch *TestChannel) Alias() string {
	return "test"
}

type TestResponse string

func (TestResponse) GetEventName() string {
	return "testResponse"
}
func (TestResponse) ProtocolAlias() string {
	return "testResponse"
}

func (ch *TestChannel) Route(ctx Context, event *EventMessage) error {
	if event.Channel != ch.Name() {
		return fmt.Errorf("unexpected channel name '%s', expecting '%s'", event.Channel, ch.Name())
	}
	if event.Name != "testRequest" {
		return fmt.Errorf("unexpected event name '%s', expecting '%s'", event.Name, "testRequest")
	}

	if string(event.Body) != "\"important message\"" {
		return fmt.Errorf("unexpected value for body: %s, expecting \"important message\"", event.Body)
	}

	if err := ctx.GetClient().Send("test", TestResponse("B")); err != nil {
		panic(fmt.Errorf("cannot send response to client: %w", err))
	}

	return nil
}

type EventMessageTest struct {
	Channel string          `json:"channel"`
	Name    string          `json:"name"`
	Body    json.RawMessage `json:"body"`
}

func TestServer(t *testing.T) {
	var s = NewServer()
	var ch = &TestChannel{}
	if err := s.Register(ch); err != nil {
		t.Fatalf("unexpected error while registering server: %s\n", err)
	}

	//run client checks
	time.AfterFunc(100*time.Millisecond, func() {

		//close the server
		defer func() {
			<-time.After(100 * time.Millisecond)
			if err := s.Close(); err != nil {
				t.Fatalf("unable to close server: %s\n", err)
			}
		}()

		conn, err := websocket.Dial("ws://localhost:34362/test", "", "http://localhost")
		if err != nil {
			t.Fatalf("unable to init websocket client: %s\n", err)
		}
		defer func() { _ = conn.Close() }()

		// send message
		if err := websocket.JSON.Send(conn, EventMessageToSend{
			Channel: "test",
			Name:    "testRequest",
			Body:    "important message",
		}); err != nil {
			t.Fatalf("unable to send message through socket: %s\n", err)
		}

		var response = EventMessageTest{}

		if err := websocket.JSON.Receive(conn, &response); err != nil {
			// handle error
			t.Fatalf("unable to receive message through socket: %s\n", err)
		}
		if response.Channel == "errors" && response.Name == "error" {
			t.Fatalf("an error occurred on server: %s", response.Body)
		}
		if response.Channel != ch.Name() {
			t.Fatalf("unexpected channel name '%s', expecting '%s'\n", response.Channel, ch.Name())
		}
		if response.Name != "testResponse" {
			t.Fatalf("unexpected event name '%s', expecting '%s'\n", response.Name, "testResponse")
		}
		if len(response.Body) != 3 {
			t.Fatalf("unexpected body length != 3\n")
		}
		if string(response.Body) != "\"B\"" {
			t.Fatalf("unexpected value for body: %s, expecting \"B\"\n", response.Body)
		}
	})
	if err := s.StartWS("/test", 34362); err != nil {
		t.Fatalf("unable to start websocket server: %s\n", err)
	}
	<-time.After(1000 * time.Millisecond)

	if !ch.GetConnected() {
		t.Fatalf("Connect() method was not called\n")
	}

	if !ch.GetDisconnected() {
		t.Fatalf("Disconnect() method was not called\n")
	}

	//message := messageType{}
	//if err := websocket.JSON.Receive(conn, &message); err != nil {
	//	// handle error
	//}

}
