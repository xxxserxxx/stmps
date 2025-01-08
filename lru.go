// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

// TODO (A) document combining Cache with LRU; is a combining wrapper needed?

// LRU is a least-recently-used algorithm for Caches. It tracks the age of items in
// a Cache by access time, and when the cache size is greater than a configured value,
// reports which items in excess of the cache size are the least recently used.
type LRU struct {
	lookup map[string]*node
	head   *node
	tail   *node
	size   int
}

type node struct {
	next  *node
	prev  *node
	value string
}

// Create a new LRU managing a map, with a given size
func NewLRU(size int) LRU {
	return LRU{
		lookup: make(map[string]*node),
		head:   nil,
		tail:   nil,
		size:   size,
	}
}

// Updates access for an item, and returns any item that
// gets pushed off the end of the LRU
func (l *LRU) Touch(key string) string {
	if n, ok := l.lookup[key]; !ok {
		newNode := &node{value: key, next: l.head}
		if l.head != nil {
			l.head.prev = newNode
		}
		l.head = newNode
	} else {
		if n.prev != nil {
			n.prev.next = n.next
		}
		if n.next != nil {
			n.next.prev = n.prev
		}
		l.head.prev = n
		n.next = l.head
		n.prev = nil
		l.head = n
	}
	if len(l.lookup) > l.size {
		remove := l.tail
		l.tail.prev.next = nil
		l.tail = l.tail.prev
		delete(l.lookup, remove.value)
		return remove.value
	}
	return ""
}
