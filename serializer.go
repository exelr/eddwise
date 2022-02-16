package eddwise

import (
	"encoding/json"
	"github.com/ugorji/go/codec"
)

type Serializer interface {
	Serialize(interface{}) ([]byte, error)
	Deserialize([]byte, interface{}) error
}

type JsonSerializer struct{}

func (s *JsonSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (s *JsonSerializer) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type MsgPackSerializer struct{}

func (s *MsgPackSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (s *MsgPackSerializer) Deserialize(data []byte, v interface{}) error {
	return codec.NewDecoderBytes(data, &codec.MsgpackHandle{}).Decode(v)
}
