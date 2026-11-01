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

func NewBoolMappedNumberFromBool(value bool) *BoolMappedNumber[uint8] {
	var intValue uint8
	if !value {
		intValue = falseValue
	} else {
		intValue = 1
	}
	return &BoolMappedNumber[uint8]{intValue}
}
