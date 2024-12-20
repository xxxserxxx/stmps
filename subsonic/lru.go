// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package subsonic

// LRU is a least-recently-used cache for maps that maintains a maximum map size.
// To use it, call Push() every time you add something to the map. If the map grows
// beyond the set size, new calls to Push will remove the oldest items from the map.
type LRU[T any] struct {
	idx        int
	ring       []string
	managedMap map[string]T
}

// Create a new LRU managing a map, with a given size
func NewLRU[T any](m map[string]T, size int) LRU[T] {
	return LRU[T]{
		idx:        0,
		ring:       make([]string, size),
		managedMap: m,
	}
}

// Push a key onto the front of the stack. If Push is called repeatedly with the
// same key, it will flush all other items out of the stack.
func (l *LRU[T]) Push(key string) {
	if _, ok := l.managedMap[l.ring[l.idx]]; ok {
		delete(l.managedMap, l.ring[l.idx])
	}
	if l.idx < len(l.ring)-1 {
		l.idx++
	} else {
		l.idx = 0
	}
	l.ring[l.idx] = key
}
