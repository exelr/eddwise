package main

import (
	"fmt"
	"log"
	"sync"

	"chat/gen/chat"

	"github.com/Pallinder/go-randomdata"
	"github.com/exelr/eddwise"
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

func (ch *ChatChannel) Connected(c *eddwise.Client) error {
	fmt.Println("User connected", c.GetId())
	var name = randomdata.SillyName()
	ch.mx.Lock()
	ch.users[c.GetId()] = name
	ch.mx.Unlock()
	_ = ch.BroadcastUserEnter(ch.GetServer().GetClients(c.GetId()), &chat.UserEnter{
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

func (ch *ChatChannel) Disconnected(c *eddwise.Client) error {
	fmt.Println("User disconnected", c.GetId(), ch.GetName(c.GetId()))
	ch.mx.Lock()
	delete(ch.users, c.GetId())
	ch.mx.Unlock()

	_ = ch.BroadcastUserLeft(ch.GetServer().GetClients(c.GetId()), &chat.UserLeft{
		UserId: c.GetId(),
	})

	return nil
}

func (ch *ChatChannel) OnMessage(ctx chat.ChatContext, evt *chat.Message) error {
	fmt.Println("Received message from", ctx.GetClient().GetId(), ":", evt.Text)
	var targets = ctx.GetServer().GetClients(ctx.GetClient().GetId())
	var id = ctx.GetClient().GetId()
	_ = ctx.GetChannel().BroadcastMessage(targets, &chat.Message{
		UserId: &id,
		Text:   evt.Text,
	})
	return nil
	//return ctx.GetChannel().Send(ctx.GetClient(), &pingpong.Pong{ID: ping.ID})
}

func (ch *ChatChannel) OnChangeName(ctx chat.ChatContext, evt *chat.ChangeName) error {
	fmt.Println("Received change name from", ctx.GetClient().GetId(), ":", evt.Name)
	var targets = ctx.GetServer().GetClients(ctx.GetClient().GetId())
	var id = ctx.GetClient().GetId()
	_ = ctx.GetChannel().BroadcastChangeName(targets, &chat.ChangeName{
		UserId: &id,
		Name:   evt.Name,
	})
	return nil
}

func main() {
	var server = eddwise.NewServer()
	var ch = NewChatChannel()
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service: ", err)
	}
	log.Fatalln(server.StartWS("/chat", 3000))
}
