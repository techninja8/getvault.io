package sharding

import (
	"fmt"
	"sync"
)

// ShardStore defines the interface for storing and retrieving shards.
type ShardStore interface {
	StoreShard(dataID string, index int, shard []byte) error
	RetrieveShard(dataID string, index int) ([]byte, error)
}

// InMemoryShardStore is a simple implementation using a map.
type InMemoryShardStore struct {
	store map[string]map[int][]byte
	mu    sync.RWMutex
}

func NewInMemoryShardStore() *InMemoryShardStore {
	return &InMemoryShardStore{
		store: make(map[string]map[int][]byte),
	}
}

func (ims *InMemoryShardStore) StoreShard(dataID string, index int, shard []byte) error {
	ims.mu.Lock()
	defer ims.mu.Unlock()
	if _, exists := ims.store[dataID]; !exists {
		ims.store[dataID] = make(map[int][]byte)
	}
	ims.store[dataID][index] = shard
	fmt.Printf("Stored shard %d for DataID: %s\n", index, dataID)
	return nil
}

func (ims *InMemoryShardStore) RetrieveShard(dataID string, index int) ([]byte, error) {
	ims.mu.RLock()
	defer ims.mu.RUnlock()
	shards, exists := ims.store[dataID]
	if !exists {
		return nil, fmt.Errorf("no shards found for DataID: %s", dataID)
	}
	shard, exists := shards[index]
	if !exists {
		return nil, fmt.Errorf("shard %d not found for DataID: %s", index, dataID)
	}
	fmt.Printf("Retrieved shard %d for DataID: %s\n", index, dataID)
	return shard, nil
}
