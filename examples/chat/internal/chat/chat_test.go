package chat

import (
	"testing"

	"chat/gen/chat/behave"

	"github.com/exelr/eddwise"
)

//Note: for the best BDD use command 'goconvey'

func TestBasicScenarioChat(t *testing.T) {
	var behave = chatbehave.NewChatBehave(t)
	behave.Given("an empty Chat channel", func() eddwise.ImplChannel { return NewChatChannel() }, func() {
		var ch = behave.Recv().(*ChatChannel)
		_ = ch // check ch status in test!
		behave.ThenClientJoins(1, func() {
			// after client joins, something would happen...
			// behave.ThenClientShouldReceiveEvent("with id 1", 1, &chat.Welcome{})
			// you can also use goconvey.Convey() to test more complex behavioural patterns
		})

	})
}
