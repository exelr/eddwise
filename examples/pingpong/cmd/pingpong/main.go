package main

import (
	"log"
	"time"

	"pingpongyaml/gen/pingpong"

	"github.com/exelr/eddwise"
)

type PingpongChannel struct {
	pingpong.Pingpong
	waitBeforeRePing time.Duration
}

func NewPingpongChannel(waitBeforeRePing time.Duration) *PingpongChannel {
	return &PingpongChannel{waitBeforeRePing: waitBeforeRePing}
}

func (ch *PingpongChannel) Connected(c eddwise.Client) error {
	//log.Println("User connected", c.GetId())
	_ = ch.SendPing(c, &pingpong.Ping{Id: 1})
	return nil
}

func (ch *PingpongChannel) Disconnected(c eddwise.Client) error {
	//log.Println("User disconnected", c.GetId())
	return nil
}

func (ch *PingpongChannel) OnPong(ctx eddwise.Context, pong *pingpong.Pong) error {
	//log.Println("received event Pong:", pong, "from", ctx.GetClient().GetId() )
	time.AfterFunc(ch.waitBeforeRePing, func() {
		_ = ch.SendPing(ctx.GetClient(), &pingpong.Ping{Id: pong.Id + 1})
	})

	return nil
}

func main() {
	var server = eddwise.NewServer()
	var ch eddwise.ImplChannel

	ch = NewPingpongChannel(500 * time.Millisecond)
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service Pingpong: ", err)
	}

	log.Fatalln(server.StartWS("/pingpong", 3000))
}
