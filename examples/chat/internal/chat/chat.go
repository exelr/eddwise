package chat

import (
	"chat/gen/chat"
	"fmt"
	"github.com/Pallinder/go-randomdata"
	"github.com/exelr/eddwise"
	"sync"
)

type ChatChannel struct {
	chat.Chat
	users map[uint64]string
	mx    sync.RWMutex
}

func NewChatChannel() *ChatChannel {
	return &ChatChannel{
		users: map[uint64]string{},
	}
}

func (ch *ChatChannel) GetName(id uint64) string {
	ch.mx.RLock()
	defer ch.mx.RUnlock()
	return ch.users[id]
}
func (ch *ChatChannel) SetName(id uint64, name string) {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	ch.users[id] = name
}

func (ch *ChatChannel) RemoveUser(id uint64) {
	ch.mx.Lock()
	defer ch.mx.Unlock()
	delete(ch.users, id)
}

func (ch *ChatChannel) Connected(c eddwise.Client) error {
	fmt.Println("User connected", c.GetId())
	var name = randomdata.SillyName()
	ch.SetName(c.GetId(), name)
	var server = ch.GetServer()
	_ = ch.BroadcastUserEnter(server.GetClients(c.GetId()), &chat.UserEnter{
		UserId: c.GetId(),
		Name:   name,
	})
	_ = ch.SendChangeName(c, &chat.ChangeName{
		UserId: nil,
		Name:   name,
	})

	_ = ch.SendUserListUpdate(c, &chat.UserListUpdate{
		List: ch.users,
	})

	return nil
}

func (ch *ChatChannel) Disconnected(c eddwise.Client) error {
	fmt.Println("User disconnected", c.GetId(), ch.GetName(c.GetId()))
	ch.RemoveUser(c.GetId())

	_ = ch.BroadcastUserLeft(ch.GetServer().GetClients(c.GetId()), &chat.UserLeft{
		UserId: c.GetId(),
	})

	return nil
}

func (ch *ChatChannel) OnMessage(ctx eddwise.Context, evt *chat.Message) error {
	fmt.Println("Received message from", ctx.GetClient().GetId(), ":", evt.Text)
	var targets = ctx.GetServer().GetClients(ctx.GetClient().GetId())
	var id = ctx.GetClient().GetId()
	_ = ch.BroadcastMessage(targets, &chat.Message{
		UserId: &id,
		Text:   evt.Text,
	})
	return nil
	//return ctx.GetChannel().Send(ctx.GetClient(), &pingpong.Pong{ID: ping.ID})
}

func (ch *ChatChannel) OnChangeName(ctx eddwise.Context, evt *chat.ChangeName) error {
	fmt.Println("Received change name from", ctx.GetClient().GetId(), ":", evt.Name)
	ch.SetName(ctx.GetClient().GetId(), evt.Name)
	var targets = ctx.GetServer().GetClients(ctx.GetClient().GetId())
	var id = ctx.GetClient().GetId()
	_ = ch.BroadcastChangeName(targets, &chat.ChangeName{
		UserId: &id,
		Name:   evt.Name,
	})
	return nil
}
