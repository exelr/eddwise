package eddwise

import (
	"fmt"
)

type UserEvent interface {
	Event
	SetId(string)
	SetUint64Id(uint64)
}

type UserJoin struct {
	Id string `json:"id"`
}

func (*UserJoin) GetEventName() string {
	return "edd:user:join"
}

func (*UserJoin) ProtocolAlias() string {
	return "edd:user:join"
}

func (u *UserJoin) SetId(id string) {
	u.Id = id
}

func (u *UserJoin) SetUint64Id(id uint64) {
	u.Id = fmt.Sprint(id)
}

type UserLeft struct {
	Id string `json:"id"`
}

func (*UserLeft) GetEventName() string {
	return "edd:user:left"
}

func (*UserLeft) ProtocolAlias() string {
	return "edd:user:left"
}

func (u *UserLeft) SetId(id string) {
	u.Id = id
}

func (u *UserLeft) SetUint64Id(id uint64) {
	u.Id = fmt.Sprint(id)
}

type ImplChannelWithUserJoin interface {
	onJoin(ImplChannel, Client, bool) error
}

type ImplChannelWithUserLeft interface {
	onLeft(ImplChannel, Client) error
}

type ChannelBroadcastUserJoinLeft struct{}

func (chbjl *ChannelBroadcastUserJoinLeft) getOtherClientsIds(ch ImplChannel, c Client) []string {
	if chAuth, ok := ch.(ImplConnManager); ok {
		return chAuth.GetAuthorizedUserIds(c.GetRawAuth().Id)
	}
	var clients = ch.GetServer().GetClients(c.GetId())
	var ret []string
	for _, c := range clients {
		ret = append(ret, fmt.Sprint(c.GetId()))
	}
	return ret
}

func (chbjl *ChannelBroadcastUserJoinLeft) broadcast(ch ImplChannel, c Client, event UserEvent) error {
	var clients []Client
	if chAuth, ok := ch.(ImplConnManager); ok {
		event.SetId(c.GetRawAuth().Id)
		clients = chAuth.GetAuthorizedUserClients(c.GetRawAuth().Id)
	} else {
		event.SetUint64Id(c.GetId())
		clients = ch.GetServer().GetClients(c.GetId())
	}

	if len(clients) > 0 {
		return Broadcast(ch.Alias(), event, clients)
	}
	return nil
}

func (chbjl *ChannelBroadcastUserJoinLeft) onJoin(ch ImplChannel, c Client, broadcast bool) error {

	var clientIds = chbjl.getOtherClientsIds(ch, c)

	for _, id := range clientIds {
		//todo make a single event
		err := c.Send(ch.Alias(), &UserJoin{
			Id: id,
		})
		if err != nil {
			return err
		}
	}

	if broadcast {
		return chbjl.broadcast(ch, c, &UserJoin{})
	}
	return nil
}

func (chbjl *ChannelBroadcastUserJoinLeft) onLeft(ch ImplChannel, c Client) error {
	return chbjl.broadcast(ch, c, &UserLeft{})
}
