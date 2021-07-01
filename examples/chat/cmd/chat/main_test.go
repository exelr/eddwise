package main

import (
	"chat/gen/chat"
	"chat/gen/chat/behave"
	"testing"

	"github.com/exelr/eddwise"
	"github.com/smartystreets/goconvey/convey"
)

func chConstructor() eddwise.ImplChannel {
	return NewChatChannel()
}

func TestBasicScenario(t *testing.T) {
	var behave = chatbehave.NewChatBehave(t)
	behave.Given("a chat channel with no clients", chConstructor, func() {

		var ch = behave.Recv().(*ChatChannel)

		behave.ThenClientJoins(1, func() {
			convey.Convey("Then ch.users should have length 1", func() {
				convey.So(ch.users, convey.ShouldHaveLength, 1)
				behave.ThenClientShouldReceiveEvent("same as server", 1, &chat.UserListUpdate{List: ch.users})
			})
		})

		behave.ThenClientJoins(2, func() {
			convey.Convey("Then ch.users should have length 2", func() {
				convey.So(ch.users, convey.ShouldHaveLength, 2)
				behave.ThenClientShouldReceiveEvent("same as server", 2, &chat.UserListUpdate{List: ch.users})
				behave.ThenClientShouldReceiveEvent("with client 2 info", 1, &chat.UserEnter{UserId: 2, Name: ch.GetName(2)})
			})
		})

		behave.OnChangeName(1, &chat.ChangeName{
			Name: "test",
		}, func() {
			behave.ThenClientShouldReceiveEvent("Name: test", 2, &chat.ChangeName{
				UserId: behave.Client(1).GetIdP(),
				Name:   "test",
			})
			convey.Convey("Then users should contain new user name at index 1", func() {
				convey.So(ch.users[1], convey.ShouldEqual, "test")
			})
		})

		behave.OnMessage(1, &chat.Message{
			Text: "test message",
		}, func() {
			behave.ThenClientShouldReceiveEvent("Name: test", 2, &chat.Message{
				UserId: behave.Client(1).GetIdP(),
				Text:   "test message",
			})
		})

		behave.ThenRemoveClient(1, func() {
			convey.Convey("Then ch.users should have length 1", func() {
				convey.So(ch.users, convey.ShouldHaveLength, 1)
				behave.ThenClientShouldReceiveEvent("with client 1 info", 2, &chat.UserLeft{UserId: 1})
			})
		})
	})
}
