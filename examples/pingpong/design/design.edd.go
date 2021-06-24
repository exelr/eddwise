// pingpong/design/design.edd.go

// Package pingpong defines the domain of your channels and structures as "pingpong"
package pingpong

// PingPong defines the channel with its events
type PingPong interface {
	Enable(
		Ping,
		Pong,
	) //enable those messages to be propagated over the channel
	//ClientToServer(Ping) //opt. Direction of the Ping message
	//ServerToClient(Pong) //opt. Direction of the Pong message
}

// implement the events structures

type Ping struct {
	id int
}

type Pong struct {
	id int
}
