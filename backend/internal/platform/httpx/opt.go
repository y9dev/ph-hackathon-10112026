package httpx

import "encoding/json"

// Opt distinguishes an absent JSON field (Set=false) from explicit null (Null=true) and a value.
type Opt[T any] struct {
	Set  bool
	Null bool
	V    T
}

// UnmarshalJSON implements json.Unmarshaler; it is only called when the field is present.
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if string(b) == "null" {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.V)
}

// Ptr returns nil for absent/null, else a pointer to the value.
func (o Opt[T]) Ptr() *T {
	if !o.Set || o.Null {
		return nil
	}
	v := o.V
	return &v
}
