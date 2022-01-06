package json5

import (
	"bytes"
	"encoding/json"
	"io"
)

func NewDecoder(rd io.Reader) *json.Decoder {
	return json.NewDecoder(NewReader(rd))
}

func Unmarshal(data []byte, v interface{}) error {
	return NewDecoder(bytes.NewReader(data)).Decode(v)
}
