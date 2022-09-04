package eddwise

import (
	"fmt"
	"sync"
)

type Room struct {
	sync.RWMutex
	id         string
	public     bool
	ch         ImplRoomManager
	clientsMap map[uint64]Client
}

func (r *Room) clients() []Client {
	var ret = make([]Client, 0, len(r.clientsMap))
	for _, v := range r.clientsMap {
		ret = append(ret, v)
	}
	return ret
}

func (r *Room) Clients() []Client {
	r.RLock()
	defer r.RUnlock()
	return r.clients()
}

func (r *Room) Has(client Client) bool {
	r.RLock()
	defer r.RUnlock()
	_, ok := r.clientsMap[client.GetId()]
	return ok
}

func (r *Room) Join(client Client) error {
	var clients []Client
	err := func() error {
		r.Lock()
		defer r.Unlock()
		if _, ok := r.clientsMap[client.GetId()]; ok {
			return fmt.Errorf("client is already in the room")
		}
		r.clientsMap[client.GetId()] = client
		clients = r.clients()
		return nil
	}()
	if err != nil {
		return err
	}
	client.addRoom(r)
	var auth = client.GetRawAuth()
	if auth != nil {
		_ = r.ch.BroadcastRoomEvent(clients, &RoomJoin{
			Id:   client.GetRawAuth().Id,
			Room: r.id,
		})
	} else {
		_ = r.ch.BroadcastRoomEvent(clients, &RoomJoin{
			Id:   fmt.Sprint(client.GetId()),
			Room: r.id,
		})
	}
	//send list of connected players to new user
	for _, c := range clients {
		if c == client {
			continue
		}
		var auth = c.GetRawAuth()
		if auth != nil {
			_ = r.ch.SendRoomEvent(client, &RoomJoin{
				Id:   c.GetRawAuth().Id,
				Room: r.id,
			})
		} else {
			_ = r.ch.SendRoomEvent(client, &RoomJoin{
				Id:   fmt.Sprint(c.GetId()),
				Room: r.id,
			})
		}
	}
	return nil

}
func (r *Room) Left(client Client) error {
	var clients []Client
	err := func() error {
		r.Lock()
		defer r.Unlock()
		if _, ok := r.clientsMap[client.GetId()]; !ok {
			return fmt.Errorf("client is not in the room")
		}
		clients = r.clients()
		delete(r.clientsMap, client.GetId())
		return nil
	}()
	if err != nil {
		return err
	}

	client.delRoom(r)

	var auth = client.GetRawAuth()
	if auth != nil {
		_ = r.ch.BroadcastRoomEvent(clients, &RoomLeft{
			Id:   client.GetRawAuth().Id,
			Room: r.id,
		})
	} else {
		_ = r.ch.BroadcastRoomEvent(clients, &RoomLeft{
			Id:   fmt.Sprint(client.GetId()),
			Room: r.id,
		})
	}
	return nil

}

type ImplRoomManager interface {
	roomManagerInit(ch ImplChannel)
	OnRoomEvent(Client, ClientRoomEvent) error
	SendRoomEvent(Client, ServerRoomEvent) error
	BroadcastRoomEvent([]Client, ServerRoomEvent) error
	SendPublicRooms(Client) error
	RoomClientQuit(Client) error
}

type RoomManager struct {
	sync.RWMutex
	ch                    ImplChannel
	chRm                  ImplRoomManager
	rooms                 map[string]*Room
	multiRoomLimitPerUser int
	multiRoomMode         bool
}

func (rm *RoomManager) roomManagerInit(ch ImplChannel) {
	rm.ch = ch
	rm.chRm = ch.(ImplRoomManager)
	rm.rooms = make(map[string]*Room)
}

func (rm *RoomManager) SetRoomsPerUserLimit(n int) {
	rm.multiRoomLimitPerUser = n
}

func (rm *RoomManager) MultiRoomMode(b bool) {
	rm.multiRoomMode = b
}

func (rm *RoomManager) Room(id string) *Room {
	rm.RLock()
	defer rm.RUnlock()
	return rm.rooms[id]
}

func (rm *RoomManager) Create(id string, public bool) (*Room, error) {
	rm.Lock()
	defer rm.Unlock()
	if _, ok := rm.rooms[id]; ok {
		return nil, fmt.Errorf("room %s already exists", id)
	}
	var room = &Room{
		id:         id,
		ch:         rm.chRm,
		public:     public,
		clientsMap: map[uint64]Client{},
	}

	rm.rooms[id] = room
	return room, nil
}

func (rm *RoomManager) OnRoomEvent(client Client, event ClientRoomEvent) error {
	switch event := event.(type) {
	case *RoomJoinRequest:

		room := rm.Room(event.Room)
		if room == nil {
			return fmt.Errorf("unknown room %s", event.Room)
		}
		if room.Has(client) {
			return fmt.Errorf("client is already in the room")
		}
		if rm.multiRoomMode && rm.multiRoomLimitPerUser > 0 && len(client.GetRooms()) >= rm.multiRoomLimitPerUser {
			return fmt.Errorf("limit of joinable rooms reached")
		}

		if !rm.multiRoomMode {
			var activeRooms = client.GetRooms()

			if len(activeRooms) > 0 {
				if err := activeRooms[0].Left(client); err != nil {
					return err
				}
			}
		}

		return room.Join(client)
	case *RoomLeftRequest:
		room := rm.Room(event.Room)
		if room == nil {
			return fmt.Errorf("unknown room %s", event.Room)
		}
		return room.Left(client)
	case *RoomCreateRequest:
		room, err := rm.Create(event.Room, event.Public)
		if err != nil {
			return err
		}
		if event.Public {
			_ = rm.chRm.BroadcastRoomEvent(rm.ch.GetServer().GetClients(), &RoomCreate{Room: event.Room})
		} else {
			_ = rm.chRm.SendRoomEvent(client, &RoomCreate{Room: event.Room})
		}
		if err := room.Join(client); err != nil {
			return err
		}
		//var id = ""
		//var auth = client.GetRawAuth()
		//if auth != nil {
		//	id = client.GetRawAuth().Id
		//} else {
		//	id = fmt.Sprint(client.GetId())
		//}
		//_ = rm.chRm.SendRoomEvent(client, &RoomJoin{Id: id, Room: event.Room})
		return nil
	default:
		return fmt.Errorf("unknown room event %T", event)
	}
}

func (rm *RoomManager) SendRoomEvent(client Client, event ServerRoomEvent) error {
	return client.Send(rm.ch.Alias(), event)
}

func (rm *RoomManager) BroadcastRoomEvent(clients []Client, event ServerRoomEvent) error {
	return Broadcast(rm.ch.Alias(), event, clients)
}

func (rm *RoomManager) SendPublicRooms(client Client) error {
	rm.RLock()
	defer rm.RUnlock()
	for k, v := range rm.rooms {
		if v.public {
			if err := rm.chRm.SendRoomEvent(client, &RoomCreate{Room: k}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rm *RoomManager) RoomClientQuit(client Client) error {
	var rooms = client.GetRooms()
	for _, room := range rooms {
		_ = room.Left(client)
	}
	return nil
}
