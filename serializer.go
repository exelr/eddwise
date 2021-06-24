package eddwise

import "encoding/json"

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
