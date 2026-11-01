package internal

import (
	"encoding/json"
	"iter"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
)

type SafeMap[K comparable, V any] struct {
	lock sync.Mutex
	wo   map[K]V
	ro   atomic.Pointer[map[K]V]
}

func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	sf := &SafeMap[K, V]{
		wo: make(map[K]V),
	}
	sf.updateReadOnly()
	return sf
}

func (m *SafeMap[K, V]) updateReadOnly() {
	m.ro.Store(new(maps.Clone(m.wo)))
}

func (m *SafeMap[K, V]) Load(key K) (value V, ok bool) {
	ro := m.ro.Load()
	value, ok = (*ro)[key]
	return
}

func (m *SafeMap[K, V]) StoreAndDelete(storeKey K, storeValue V, deleteKey K) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.wo[storeKey] = storeValue
	delete(m.wo, deleteKey)
	m.updateReadOnly()
}

func (m *SafeMap[K, V]) Store(key K, value V, replace func(stored V) bool) (stored V, exists bool) {
	if replace == nil {
		replace = func(_ V) bool {
			return true
		}
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if stored, exists = m.wo[key]; !exists || replace(stored) {
		stored = value
		m.wo[key] = stored
		m.updateReadOnly()
	}
	return
}

func (m *SafeMap[K, V]) Delete(key K) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.wo, key)
	m.updateReadOnly()
}

func (m *SafeMap[K, V]) CompareAndDelete(key K, compareFunc func(stored V) bool) (deleted bool) {
	if compareFunc == nil {
		compareFunc = func(_ V) bool {
			return false
		}
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if current, ok := m.wo[key]; ok && compareFunc(current) {
		delete(m.wo, key)
		m.updateReadOnly()
		deleted = true
	}
	return
}

func (m *SafeMap[K, V]) LoadOrStoreFn(key K, valueFn func() V) (actual V, loaded bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if actual, loaded = m.wo[key]; !loaded {
		value := valueFn()
		m.wo[key] = value
		m.updateReadOnly()
		actual = value
	}
	return
}

func (m *SafeMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		roMap := m.ro.Load()
		for _, value := range *roMap {
			if !yield(value) {
				break
			}
		}
	}
}

func (m *SafeMap[K, V]) Iter() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		roMap := m.ro.Load()
		for key, value := range *roMap {
			if !yield(key, value) {
				break
			}
		}
	}
}

func (m *SafeMap[K, V]) Len() int {
	return len(*m.ro.Load())
}

func (m *SafeMap[K, V]) MarshalJSON() (b []byte, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return json.Marshal(m.wo)
}

func (m *SafeMap[K, V]) UnmarshalJSON(b []byte) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if err = json.Unmarshal(b, &m.wo); err != nil {
		return err
	}
	m.updateReadOnly()
	return nil
}

type SafeSet[V comparable] struct {
	safeMap *SafeMap[V, any]
}

func NewSafeSet[V comparable]() *SafeSet[V] {
	return &SafeSet[V]{
		safeMap: NewSafeMap[V, any](),
	}
}

func (s *SafeSet[V]) Delete(value V) bool {
	return s.safeMap.CompareAndDelete(value, func(_ any) bool {
		return true
	})
}

func (s *SafeSet[V]) Store(value V) bool {
	_, exists := s.safeMap.Store(value, nil, func(_ any) bool {
		return false
	})
	return !exists
}

func (s *SafeSet[V]) Len() int {
	if s == nil {
		return 0
	}
	return s.safeMap.Len()
}

type ReadOnlyOrderedMap[K comparable, V any] struct {
	internal map[K]V
	keys     []K
}

func NewReadOnlyOrderedMap[K comparable, V any](keyOrder []K, mapping map[K]V) *ReadOnlyOrderedMap[K, V] {
	return &ReadOnlyOrderedMap[K, V]{
		internal: mapping,
		keys:     keyOrder,
	}
}

func (m *ReadOnlyOrderedMap[K, V]) Load(key K) (value V, ok bool) {
	value, ok = m.internal[key]
	return
}

func (m *ReadOnlyOrderedMap[K, V]) Len() int {
	return len(m.keys)
}

func (m *ReadOnlyOrderedMap[K, V]) Iter() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, key := range m.keys {
			if !yield(key, m.internal[key]) {
				break
			}
		}
	}
}

func (m *ReadOnlyOrderedMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, key := range m.keys {
			if !yield(m.internal[key]) {
				break
			}
		}
	}
}

type baseSafeOrderedMapData[K comparable, V any] struct {
	internal map[K]V
	keys     []K
}

type safeOrderedMapData[K comparable, V any] struct {
	*baseSafeOrderedMapData[K, V]
	keyMap map[K]int
}

type SafeOrderedMap[K comparable, V any] struct {
	lock sync.Mutex
	data *safeOrderedMapData[K, V]
	ro   atomic.Pointer[baseSafeOrderedMapData[K, V]]
}

func NewSafeOrderedMap[K comparable, V any]() *SafeOrderedMap[K, V] {
	m := &SafeOrderedMap[K, V]{
		data: &safeOrderedMapData[K, V]{
			baseSafeOrderedMapData: &baseSafeOrderedMapData[K, V]{
				internal: make(map[K]V),
			},
			keyMap: make(map[K]int),
		},
	}
	m.updateReadOnly()
	return m
}

func (m *SafeOrderedMap[K, V]) updateReadOnly() {
	m.ro.Store(&baseSafeOrderedMapData[K, V]{
		internal: maps.Clone(m.data.internal),
		keys:     slices.Clone(m.data.keys),
	})
}

func (m *SafeOrderedMap[K, V]) store(key K, value V, replace func(stored V) bool) (stored bool, storedValue V) {
	if replace == nil {
		replace = func(_ V) bool {
			return true
		}
	}
	if storedValue, stored = m.data.internal[key]; !stored {
		storedValue = value
		m.data.keyMap[key] = len(m.data.keys)
		m.data.keys = append(m.data.keys, key)
		m.data.internal[key] = value
		m.updateReadOnly()
	} else if replace(storedValue) {
		storedValue = value
		m.data.internal[key] = storedValue
		m.updateReadOnly()
	}
	return
}

func (m *SafeOrderedMap[K, V]) Load(key K) (value V, ok bool) {
	ro := m.ro.Load()
	value, ok = ro.internal[key]
	return
}

func (m *SafeOrderedMap[K, V]) Len() int {
	ro := m.ro.Load()
	return len(ro.keys)
}

func (m *SafeOrderedMap[K, V]) Store(key K, value V, replace func(stored V) bool) (stored bool, storedValue V) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.store(key, value, replace)
}

func (m *SafeOrderedMap[K, V]) Delete(key K) bool {
	var exists bool
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, exists = m.data.internal[key]; exists {
		index := m.data.keyMap[key]
		delete(m.data.keyMap, key)
		m.data.keys = slices.Delete(m.data.keys, index, index+1)
		delete(m.data.internal, key)
		for k, idx := range m.data.keyMap {
			if idx > index {
				m.data.keyMap[k] = idx - 1
			}
		}
		m.updateReadOnly()
	}
	return exists
}

func (m *SafeOrderedMap[K, V]) IterAndStore(key K, value V, replace func(stored V) bool, fn func(int, iter.Seq2[K, V])) (stored bool, storedValue V) {
	m.lock.Lock()
	defer m.lock.Unlock()
	length, iterable := m.data.iter()
	fn(length, iterable)
	return m.store(key, value, replace)
}

func (m *baseSafeOrderedMapData[K, V]) iter() (int, iter.Seq2[K, V]) {
	return len(m.keys), func(yield func(K, V) bool) {
		for _, key := range m.keys {
			if !yield(key, m.internal[key]) {
				break
			}
		}
	}
}

func (m *SafeOrderedMap[K, V]) Keys() (int, iter.Seq[K]) {
	ro := m.ro.Load()
	return len(ro.keys), func(yield func(K) bool) {
		for _, key := range ro.keys {
			if !yield(key) {
				break
			}
		}
	}
}

func (m *SafeOrderedMap[K, V]) Values() (int, iter.Seq[V]) {
	ro := m.ro.Load()
	return len(ro.keys), func(yield func(V) bool) {
		for _, key := range ro.keys {
			if !yield(ro.internal[key]) {
				break
			}
		}
	}
}

func (m *SafeOrderedMap[K, V]) Iter() (int, iter.Seq2[K, V]) {
	ro := m.ro.Load()
	return ro.iter()
}

func (m *SafeOrderedMap[K, V]) First() (key K, value V, ok bool) {
	ro := m.ro.Load()
	if len(ro.keys) == 0 {
		return
	}
	key = ro.keys[0]
	value = ro.internal[key]
	ok = true
	return
}
