package eddwise

type ClientRoomEvent interface {
	Event
	ClientRoomEvent()
}

type RoomCreateRequest struct {
	Room   string `json:"room"`
	Public bool   `json:"public"`
}

func (*RoomCreateRequest) ClientRoomEvent() {}

func (*RoomCreateRequest) GetEventName() string {
	return "edd:room:create_request"
}

func (*RoomCreateRequest) ProtocolAlias() string {
	return "edd:room:create_request"
}

type RoomJoinRequest struct {
	Room string `json:"room"`
}

func (*RoomJoinRequest) ClientRoomEvent() {}

func (*RoomJoinRequest) GetEventName() string {
	return "edd:room:join_request"
}

func (*RoomJoinRequest) ProtocolAlias() string {
	return "edd:room:join_request"
}

type RoomLeftRequest struct {
	Room string `json:"room"`
}

func (*RoomLeftRequest) ClientRoomEvent() {}

func (*RoomLeftRequest) GetEventName() string {
	return "edd:room:left_request"
}

func (*RoomLeftRequest) ProtocolAlias() string {
	return "edd:room:left_request"
}

type ServerRoomEvent interface {
	Event
	ServerRoomEvent()
}

type RoomCreate struct {
	Room string `json:"room"`
}

func (*RoomCreate) ServerRoomEvent() {}

func (*RoomCreate) GetEventName() string {
	return "edd:room:create"
}

func (*RoomCreate) ProtocolAlias() string {
	return "edd:room:create"
}

type RoomJoin struct {
	Id   string `json:"id"`
	Room string `json:"room"`
}

func (*RoomJoin) ServerRoomEvent() {}

func (*RoomJoin) GetEventName() string {
	return "edd:room:join"
}

func (*RoomJoin) ProtocolAlias() string {
	return "edd:room:join"
}

type RoomLeft struct {
	Id   string `json:"id"`
	Room string `json:"room"`
}

func (*RoomLeft) ServerRoomEvent() {}

func (*RoomLeft) GetEventName() string {
	return "edd:room:left"
}

func (*RoomLeft) ProtocolAlias() string {
	return "edd:room:left"
}
