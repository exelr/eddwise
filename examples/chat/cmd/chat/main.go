package main

import (
	"log"

	"chat/internal/chat"

	"github.com/exelr/eddwise"
)

func main() {
	var server = eddwise.NewServer()
	var ch eddwise.ImplChannel

	ch = chat.NewChatChannel()
	if err := server.Register(ch); err != nil {
		log.Fatalln("unable to register service Chat: ", err)
	}

	log.Fatalln(server.StartWS("/chat", 3000))
}
