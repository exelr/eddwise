package main

import (
	"github.com/exelr/eddwise"
	"pingpongyaml/gen/pingpong"
	pingpongbehave "pingpongyaml/gen/pingpong/behave"
	"testing"
	"time"
)

func TestBasicScenario(t *testing.T) {
	var waitBeforeRePing = 500 * time.Millisecond
	var behave = pingpongbehave.NewPingpongBehave(t)
	behave.Given("an empty pingpong channel", func() eddwise.ImplChannel { return NewPingpongChannel(waitBeforeRePing) }, func() {
		//var ch = behave.Recv().(*PingpongChannel)
		behave.ThenClientJoins(1, func() {
			behave.ThenClientShouldReceiveEvent("with id 1", 1, &pingpong.Ping{Id: 1})
			behave.OnPong(1, &pingpong.Pong{Id: 1}, func() {
				behave.Waiting(waitBeforeRePing, func() {
					behave.ThenClientShouldReceiveEvent("with id 2", 1, &pingpong.Ping{Id: 2})
				})
			})
		})

	})
}
