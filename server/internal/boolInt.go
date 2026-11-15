package internal

import (
	"encoding/json"
)

const falseValue = 0

type number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~complex64 | ~complex128
}

func NumberToBool[T number](value T) bool {
	return value != falseValue
}

type BoolMappedNumber[T number] struct {
	Value T
}

func (b *BoolMappedNumber[T]) Bool() bool {
	return NumberToBool(b.Value)
}

func (b *BoolMappedNumber[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(b.Value)
}

func (b *BoolMappedNumber[T]) UnmarshalJSON(data []byte) error {
	var val T
	if err := json.Unmarshal(data, &val); err == nil {
		b.Value = val
		return nil
	} else {
		return err
	}
}

func NewBoolMappedNumber[T number](value T) *BoolMappedNumber[T] {
	return &BoolMappedNumber[T]{Value: value}
}

// NewBoolMappedNumberFromBool renders a flag as the number the game protocol
// expects, wrapped so it can travel inside a positional payload.
func NewBoolMappedNumberFromBool(value bool) *BoolMappedNumber[uint8] {
	return &BoolMappedNumber[uint8]{BoolToNumber(value)}
}

// BoolToNumber is the inverse of NumberToBool: it renders a flag as the 0/1 both
// the game protocol and the public API use for booleans. It is the single
// implementation, shared by NewBoolMappedNumberFromBool (positional payloads) and
// by the public API encoders, which put the number straight into a struct field.
func BoolToNumber(value bool) uint8 {
	if value {
		return 1
	}
	return falseValue
}

// NumberToInt64 converts a value read from a decoded JSON document, or from a
// typed store, to int64. encoding/json decodes every number as float64 while the
// in-memory resources mix that with int and int64 literals, so every positional
// reader needs this same conversion. This is the single implementation:
// models.numberToInt and the public API readers delegate here instead of each
// carrying its own type switch.
func NumberToInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		return int64(typed), true
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		return int64(typed), true
	case json.Number:
		if number, err := typed.Int64(); err == nil {
			return number, true
		}
	}
	return 0, false
}
