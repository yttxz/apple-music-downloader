package main

import (
	"fmt"
	"sync"
)

type inFlightRegistry struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func newInFlightRegistry() *inFlightRegistry {
	return &inFlightRegistry{active: make(map[string]struct{})}
}

func (r *inFlightRegistry) TryLock(item DBItem) (func(), bool) {
	if r == nil {
		return func() {}, true
	}
	key := item.inFlightKey()
	if key == "" {
		return func() {}, true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.active[key]; exists {
		return func() {}, false
	}
	r.active[key] = struct{}{}
	return func() {
		r.mu.Lock()
		delete(r.active, key)
		r.mu.Unlock()
	}, true
}

func (item DBItem) inFlightKey() string {
	if item.Storefront == "" || item.MediaType == "" || item.AppleID == "" || item.Codec == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s/%s", item.Storefront, item.MediaType, item.AppleID, item.Codec)
}

var downloadInFlight = newInFlightRegistry()
