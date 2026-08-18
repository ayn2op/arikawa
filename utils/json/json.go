// Package json allows for different implementations of JSON serializing, as
// well as extra optional types needed.
package json

import (
	"encoding/json"
	"io"
)

type Driver interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error

	DecodeStream(r io.Reader, v any) error
	EncodeStream(w io.Writer, v any) error
}

type DefaultDriver struct{}

func (d DefaultDriver) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (d DefaultDriver) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (d DefaultDriver) DecodeStream(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

func (d DefaultDriver) EncodeStream(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

// Default is the default JSON driver, which uses encoding/json.
var Default Driver = DefaultDriver{}

// Marshal uses the default driver.
func Marshal(v any) ([]byte, error) {
	return Default.Marshal(v)
}

// Unmarshal uses the default driver.
func Unmarshal(data []byte, v any) error {
	return Default.Unmarshal(data, v)
}

// DecodeStream uses the default driver.
func DecodeStream(r io.Reader, v any) error {
	return Default.DecodeStream(r, v)
}

// EncodeStream uses the default driver.
func EncodeStream(w io.Writer, v any) error {
	return Default.EncodeStream(w, v)
}
