package mock

import (
	"fmt"
	"github.com/exelr/eddwise"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

type ChannelBehave struct {
	recv   eddwise.ImplChannel
	t      *testing.T
	server *ServerMock
}

func NewBehaveChannel(t *testing.T) *ChannelBehave {
	return &ChannelBehave{
		t:      t,
		server: NewServer(),
	}
}

func (cb *ChannelBehave) NewContext(clientId uint64) eddwise.Context {
	var client = cb.server.GetClient(clientId)
	if client == nil {
		panic("invalid client id")
	}
	var ctx = eddwise.NewDefaultContextFromBackground(cb.server, client)
	return ctx
}

func (cb *ChannelBehave) SetRecv(ch eddwise.ImplChannel) {
	cb.recv = ch
}

func (cb *ChannelBehave) Recv() eddwise.ImplChannel {
	return cb.recv
}

func (cb *ChannelBehave) Given(desc string, chFn func() eddwise.ImplChannel, f func()) {
	convey.Convey("Given "+desc, cb.t, func() {
		cb.recv = chFn()
		//convey.Convey("Then no errors occurs during binding", func() {
		convey.So(cb.recv.Bind(cb.server), convey.ShouldBeNil)
		f()
		//})
	})
}

func (cb *ChannelBehave) ThenClientJoins(clientId uint64, f ...func()) {
	var err = cb.AddClient(clientId)
	convey.Convey(fmt.Sprintf("Then a client with id %d joins", clientId), func() {
		convey.So(err, convey.ShouldBeNil)
		if len(f) > 0 {
			f[0]()
		}
	})
}

func (cb *ChannelBehave) ThenClientCannotJoins(clientId uint64, f ...func()) {
	var err = cb.AddClient(clientId)
	convey.Convey(fmt.Sprintf("Then a client with id %d cannot join", clientId), func() {
		convey.So(err, convey.ShouldNotBeNil)
		if len(f) > 0 {
			f[0]()
		}
	})
}

func (cb *ChannelBehave) ThenRemoveClient(clientId uint64, f ...func()) {
	var err = cb.RemoveClient(clientId)
	convey.Convey(fmt.Sprintf("Then a client with id %d should be removed", clientId), func() {
		convey.So(err, convey.ShouldBeNil)
		if len(f) > 0 {
			f[0]()
		}
	})
}

func (cb *ChannelBehave) Client(clientId uint64) *Client {
	return cb.server.GetClient(clientId).(*Client)
}

func (cb *ChannelBehave) ThenClientShouldReceiveEvent(info string, clientId uint64, event eddwise.Event) {
	var c = cb.server.GetClient(clientId).(*Client)
	convey.So(c, convey.ShouldNotBeNil)
	convey.Convey(fmt.Sprintf("Then client %d should receive %s (%s)", c.GetId(), event.GetEventName(), info), func() {
		convey.So(c.HasEvent(event), convey.ShouldBeTrue)
	})
}

func (cb *ChannelBehave) AddClient(clientId uint64) error {
	var client = NewClient(clientId)
	cb.server.AddClient(client)
	if c, ok := cb.recv.(eddwise.ImplChannelConnected); ok {
		return c.Connected(client)
	}
	return nil
}
func (cb *ChannelBehave) RemoveClient(clientId uint64) error {
	var client = cb.server.GetClient(clientId)
	defer cb.server.RemoveClient(client)
	if c, ok := cb.recv.(eddwise.ImplChannelDisconnected); ok {
		return c.Disconnected(client)
	}
	return nil
}

func (cb *ChannelBehave) ThenChannelStateShouldBe(channel eddwise.ImplChannel) {
	convey.Convey(fmt.Sprintf("Then server should be"), func() {
		convey.So(channel, convey.ShouldEqual, cb.recv)
	})
}

func (cb *ChannelBehave) On(clientId uint64, onEvt func(ctx eddwise.Context) error, evt eddwise.Event, f ...func()) {
	var ctx = cb.NewContext(clientId)
	convey.Convey(fmt.Sprintf("On event %s - %+v", evt.GetEventName(), evt), func() {
		var err = onEvt(ctx)
		convey.So(err, convey.ShouldBeNil)
		if len(f) > 0 {
			f[0]()
		}
	})
}
