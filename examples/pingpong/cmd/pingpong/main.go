package main

import (
	"log"
	"time"

	"pingpong/gen/pingpong"

	"github.com/exelr/eddwise"
)

type PingPongChannel struct {
	pingpong.PingPong
}

func (ch *PingPongChannel) OnPing(ctx pingpong.PingPongContext, ping *pingpong.Ping) error {
	return ch.SendPong(ctx.GetClient(), &pingpong.Pong{Id: ping.Id})
}

func (ch *PingPongChannel) OnPong(ctx pingpong.PingPongContext, pong *pingpong.Pong) error {
	time.AfterFunc(1*time.Second, func() {
		_ = ch.SendPing(ctx.GetClient(), &pingpong.Ping{Id: pong.Id + 1})
	})
	return nil
}

func (ch *PingPongChannel) Connected(c *eddwise.Client) error {
	return ch.SendPing(c, &pingpong.Ping{Id: 1})
}

func main() {
	var server = eddwise.NewServer()
	var ch = &PingPongChannel{}
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service PingPong: ", err)
	}
	log.Fatalln(server.StartWS("/pingpong", 3000))
}
