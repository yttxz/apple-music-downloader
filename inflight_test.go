package main

import "testing"

func TestInFlightRegistryTryLock(t *testing.T) {
	registry := newInFlightRegistry()
	item := DBItem{Storefront: "us", MediaType: "song", AppleID: "1", Codec: "ALAC"}

	unlock, ok := registry.TryLock(item)
	if !ok {
		t.Fatal("first TryLock failed")
	}
	if _, ok := registry.TryLock(item); ok {
		t.Fatal("second TryLock succeeded while item was active")
	}
	unlock()
	if _, ok := registry.TryLock(item); !ok {
		t.Fatal("TryLock failed after unlock")
	}
}

func TestInFlightRegistryIgnoresIncompleteKeys(t *testing.T) {
	registry := newInFlightRegistry()
	if _, ok := registry.TryLock(DBItem{AppleID: "1"}); !ok {
		t.Fatal("incomplete key should not block")
	}
}
