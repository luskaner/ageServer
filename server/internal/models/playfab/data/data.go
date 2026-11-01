package data

import (
	"encoding/json"
	"time"
)

const CustomTimeFormat = "2006-01-02T15:04:05.000Z"

type CustomTime struct {
	time.Time
	Format string
}

func (ct *CustomTime) Update() {
	ct.Time = time.Now()
}

func (ct CustomTime) MarshalJSON() ([]byte, error) {
	formatted := ct.Time.UTC().Format(CustomTimeFormat)
	return json.Marshal(formatted)
}

type Value[T any] struct {
	LastUpdated CustomTime
	Permission  string
	Value       *T
}

func (v *Value[T]) MarshalJSON() ([]byte, error) {
	if val, err := json.Marshal(v.Value); err == nil {
		return json.Marshal(BaseValue[string]{
			LastUpdated: v.LastUpdated,
			Permission:  v.Permission,
			Value:       new(string(val)),
		})
	} else {
		return nil, err
	}
}

type BaseValue[T any] Value[T]

func (b *BaseValue[T]) UpdateLastUpdated() {
	b.LastUpdated.Update()
}

func (b *BaseValue[T]) Update(updateFn func(*T)) {
	updateFn(b.Value)
	b.UpdateLastUpdated()
}

func (b *BaseValue[T]) ToValue() *Value[T] {
	if b == nil {
		return nil
	}
	return &Value[T]{
		LastUpdated: b.LastUpdated,
		Permission:  b.Permission,
		Value:       b.Value,
	}
}

func NewBaseValue[T any](permission string, value T) *BaseValue[T] {
	return &BaseValue[T]{
		LastUpdated: CustomTime{Time: time.Now(), Format: "2006-01-02T15:04:05.000Z"},
		Permission:  permission,
		Value:       &value,
	}
}

func NewPrivateBaseValue[T any](value T) *BaseValue[T] {
	return NewBaseValue("Private", value)
}
