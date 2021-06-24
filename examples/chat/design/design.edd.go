package chat // Package chat defines the domain as "chat"

// Chat defines the channel with its events
type Chat interface {
	Enable(
		ChangeName,
		Message,
		UserEnter,
		UserLeft,
		UserListUpdate,
	) //enable those messages to be propagated over the channel
	ServerToClient(UserEnter, UserLeft, UserListUpdate) //prevent those events to be sent from clients
}

// ChangeName event is triggered by clients and broadcasted to other clients by server
type ChangeName struct {
	// userId is the optional id of the user. Set only by server, not by client
	userId *uint64
	name   string
}

type Message struct {
	// userId is the optional id of the user. Set only by server, not by client
	userId *uint64
	text   string
}

type UserEnter struct {
	userId uint64
	// name is the name
	name string
}

type UserLeft struct {
	userId uint64
}

// UserListUpdate contains an updated list with connected user
type UserListUpdate struct {
	list map[uint64]string
}
