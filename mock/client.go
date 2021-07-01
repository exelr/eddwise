package mock

import (
	"errors"
	"github.com/exelr/eddwise"
	"reflect"
	"time"
)

type RecordedEvent struct {
	Event     eddwise.Event
	Channel   string
	Timestamp time.Time
}

type Client struct {
	id     uint64
	closed bool
	events []RecordedEvent
}

func NewClient(id uint64) *Client {
	return &Client{
		id:     id,
		closed: false,
		events: nil,
	}
}
func (cm *Client) GetId() uint64 {
	return cm.id
}
func (cm *Client) GetIdP() *uint64 {
	var id = cm.id
	return &id
}
func (cm *Client) Send(channel string, event eddwise.Event) error {
	cm.events = append(cm.events, RecordedEvent{
		Event:     event,
		Channel:   channel,
		Timestamp: time.Now(),
	})
	return nil
}
func (cm *Client) SendJSON(interface{}) error {
	return errors.New("not implemented in mock")
}
func (cm *Client) Close() error {
	cm.closed = true
	return nil
}
func (cm *Client) Closed() bool {
	return cm.closed
}

func (cm *Client) Recorded() []RecordedEvent {
	return cm.events
}

func (cm *Client) HasEvent(event eddwise.Event) bool {
	//defer func(){
	//	cm.events = cm.events[:]
	//}()
	for _, rec := range cm.events {
		if rec.Event.GetEventName() == event.GetEventName() {
			if reflect.DeepEqual(event, rec.Event) {
				return true
			}
		}
	}
	return false
}
