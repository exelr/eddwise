
# Edd WiSe

![Release](https://img.shields.io/github/v/release/exelr/eddwise.svg)
![Test](https://github.com/exelr/eddwise/workflows/Test/badge.svg)
![Security](https://github.com/exelr/eddwise/workflows/Security/badge.svg)
![Linter](https://github.com/exelr/eddwise/workflows/Linter/badge.svg)

<div>
<img align="left" src="https://raw.githubusercontent.com/exelr/eddwise/master/logo.png" alt="Edd WiSe" width="100" height="100" />
<br/>
<p>
	Edd WiSe - <b>E</b>vent <b>d</b>riven <b>d</b>esign over <b>W</b>eb <b>S</b>ocket<br />
    A tool to design uni or bi-directional event driven web applications. <br />
</p>
<p><br /></p>
</div>

## Design First

You provide a simple description of your service within a yaml file,
and you are able to generate **documented** code of both server (Golang) and client (Javascript). A dummy server/client implementations will be generated too, so you can fill them with business logic.

## Behavioural Second

Speaking about events, it is natural to think how an event would influence the state of your application and of your remote clients.

Also, with the server code, the generation tool provides you with a "behave" interface that let you to naturally implement BDD scenarios thanks to a leveraging of [goconvey](https://github.com/smartystreets/goconvey) around your defined events.

## Install

Download the latest version for your OS from [releases](https://github.com/exelr/eddwise/releases) or install it from go:

```shell
go install github.com/exelr/eddwise/cmd/edd
```

## Minimal design

Define your design:
```yaml
# design/pingpong.edd.yml
namespace: pingpong
structs:
  ping:
    id: uint
  pong:
    id: uint

channels:
  pingpong:
    client:
      - ping 
    server:
      - pong
```
<details>
  <summary>View details</summary>

```yaml
# design/pingpong.edd.yml
namespace: pingpong # the namespace of your generated code (packages for go)
structs:
  ping: # ping is emitted from client
    id: uint # the id of the ping
  pong: # pong is sent from server after a ping
    id: uint # the id of the pong, same as the id of the received ping

channels:
  pingpong: # create a channel named pingpong
    client: # define the events that can pass through the channel pingpong generated from client
      - ping # set ping to be originated only from client 
    server: # define events generated from server
      - pong # set pong to be originated only from server
#    dual: # optional, you can define the events that are generated in both client and server
#      - ping
#      - pong
```
</details>

Generate the code:

```shell
edd design gen
```

## Simple library

Server:
```go
// cmd/pingpong/main.go
package main

import (
    "log"

    "github.com/exelr/eddwise"
    "pingpong/gen/pingpong"
)

type PingPongChannel struct {
    pingpong.PingPong
}

func (ch *PingPongChannel) OnPing(ctx eddwise.Context, ping *pingpong.Ping) error {
    return ch.SendPong(ctx.GetClient(), &pingpong.Pong{Id: ping.Id})
}

func main(){
    var server = eddwise.NewServer()
    var ch = &NewPingPongChannel{}
    if err := server.Register(ch); err != nil {
        log.Fatalln("unable to register service PingPong: ", err)
    }
    log.Fatalln(server.StartWS("/pingpong", 3000))
}

```
Client:
```html
// web/pingpong/app.html
<script type="module">
    import {EddClient} from '/pingpong/edd.js'
    import {pingpongChannel} from '../../gen/pingpong/channel.js'
    let client = new EddClient("ws://localhost:3000/pingpong")
    let pingpong = new pingpongChannel()
    client.register(pingpong)
    
    pingpong.onpong((pong) => {
        console.log("received pong with id", pong.id)
    });
    
    let pinginterval;
    pingpong.connected(() => {
        let id = 1;
        pinginterval = setInterval(function () {
            pingpong.sendping({id: id++})
        }, 1000)
    })
    pinginterval.disconnected = function(){
        clearInterval(pinginterval)
    }
</script>
```

You can also generate skeleton code for client and server directly:

```shell
edd design skeleton
```

### Want to see more?

See [Examples repo](https://github.com/exelr/eddwise-examples).

A full demo of a simple web game is available, see [Filotto](https://github.com/exelr/filotto).

## Alternatives

- GRPC over websocket
- AsyncAPI with websocket extension
- WAMP (for the ws not for the design)
- [Flogo](https://github.com/TIBCOSoftware/flogo)

Logo from [gopherize.me](https://gopherize.me)
