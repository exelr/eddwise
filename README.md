# Edd WiSe

![Release](https://img.shields.io/github/v/release/exelr/eddwise.svg)
![Test](https://github.com/exelr/eddwise/workflows/Test/badge.svg)
![Security](https://github.com/exelr/eddwise/workflows/Security/badge.svg)
![Linter](https://github.com/exelr/eddwise/workflows/Linter/badge.svg)

Edd WiSe - **E**vent **d**riven **d**esign over **W**eb **S**ocket - is a tool to design uni or bi-directional event driven web applications.

You can provide a simple description of your service with a subset of Golang syntax,
and you are able to generate **documented** code of both server (Golang) and client (Javascript), and dummy server/client implementations to be filled with business logic.


## Install

Download the latest version for your OS from [releases](https://github.com/exelr/eddwise/releases) or install it from go:

```shell
go install github.com/exelr/eddwise/cmd/edd
```

## Simple design

Define your design:

```go
// design/design.edd.go

package pingpong // Package pingpong defines the domain as "pingpong"

// PingPong defines the channel with its events
type PingPong interface {
    Enable (
        Ping, 
        Pong,
    ) //enable those messages to be propagated over the channel
    ClientToServer(Ping) //opt. Direction of the Ping message
    ServerToClient(Pong) //opt. Direction of the Pong message
}

// implement the events structures

type Ping struct {
    id int
}

type Pong struct {
    id int
}
```

Generate the code:

```shell
edd pingpong/design gen
```

## Powerful library

Server:
```go
// cmd/pingpong/main.go
package main

import (
    "log"

    "github.com/hacktales/eddwise"
    "pingpong/gen/pingpong"
)

type PingPongChannel struct {
    pingpong.PingPong
}

func (ch *PingPongChannel) OnPing(ctx *pingpong.PingPongContext, ping *pingpong.Ping) error {
    return ch.SendPong(ctx.GetClient(), &pingpong.Pong{Id: ping.Id})
}

func main(){
    var server = eddwise.NewServer()
    var ch = &PingPongChannel{}
    if err := server.Register(ch); err != nil {
        log.Fatalln("unable to register service PingPong: ", err)
    }
    log.Fatalln(server.StartWS("/pingpong", 3000))
}

```
Client:
```html
// web/pingpong/app.html
<script src="//localhost:3000/pingpong/edd.js"></script>
<script src="gen/pingpong/channel.js"></script>
<script>
    let client = new EddClient("ws://localhost:3000/pingpong")
    let pingpong = new PingPongChannel()
    client.register(pingpong)
    
    pingpong.onPong((pong) => {
        console.log("received pong with id", pong.id)
    });
    
    let pinginterval;
    pingpong.connected(() => {
        let id = 1;
        pinginterval = setInterval(function () {
            pingpong.sendPing({id: id++})
        }, 1000)
    })
    pinginterval.disconnected = function(){
        clearInterval(pinginterval)
    }
</script>
```

You can also generate skeleton code for client and server directly:

```shell
edd pingpong/design skeleton
```

### Want to see more?

See [Examples directory](examples).

A full demo of a simple web game is available, see [Filotto](https://github.com/exelr/filotto).

## Why not pub/sub for an event driven design system ?

Mainly because publish and subscribe adds a layer between channel and events,
in particular for any defined event you have to associate it to at least a publish or a subscription (to make sense of its existence).
Instead Edd WiSe define a channel and messages that can go trough the channel, associating to the message an optional direction (Server->Client or Client->Server).
So the actual relation of "publish" and "subscribe" (or both) is an explicit design direction (or the lack of it) of the event in the channel.

Take the example of pingpong service above:
- In pub/sub pattern you have to define the pub and the sub part of the channel, but you are not defining if the client nor the server is the consumer or the publisher, so you have to do it in the business logic layer, or have to define an additional layer of metadata, or configuration layer that you will be setup next (it is ok in different scenarios, especially where queues are involved).
- Eddwise just use directions to enforce the consumer or the publisher of a particular event.
  Publishers and subscribers can be both client and server concurrently.
  Considering that the main use of Edd WiSe is for frontend<->backend direct communication, I feel the approach more simple and reactive wrt pub/sub.


If you want to keep the channel more abstract, say you want to reply to a pong when a ping is received regardless of the source of the incoming ping, you can just drop the direction of your events in the channel,
and implement on both client and server the pong transmission after ping reception, and the timed ping transmission.

## Why Golang DSL?

The main reason is because Golang interfaces and structures are strong typed and easy to validate and generate code from them (thanks to go/ast <3).
In the future we can use more appropriate DSL to design (jsonschema/protobuf/...?) or evolve the actual one.

## Alternatives

- GRPC over websocket
- AsyncAPI with websocket extension
- WAMP (for the ws not for the design)
