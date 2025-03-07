package sharding

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type ShardStore interface {
	StoreShard(dataID string, index int, shard []byte, location string) error
	RetrieveShard(dataID string, index int, location string) ([]byte, error)
}

// InMemoryShardStore with file persistence
type InMemoryShardStore struct {
	ShardStore map[string]map[int][]byte
	mu         sync.RWMutex
}

func NewInMemoryShardStore() *InMemoryShardStore {
	/// This should leave a message
	store := &InMemoryShardStore{
		ShardStore: make(map[string]map[int][]byte),
	}
	return store
}

// StoreShard stores a shard and persists it to disk
func (ims *InMemoryShardStore) StoreShard(dataID string, index int, shard []byte, location string) error {
	ims.mu.Lock()
	defer ims.mu.Unlock()

	// Store in memory
	if _, exists := ims.ShardStore[dataID]; !exists {
		ims.ShardStore[dataID] = make(map[int][]byte)
	}
	ims.ShardStore[dataID][index] = shard

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(location, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Store to disk
	if err := ims.writeShardToDisk(dataID, index, shard, location); err != nil {
		return fmt.Errorf("failed to persist shard: %w", err)
	}

	fmt.Printf("Stored shard %d for DataID: %s in location: %s\n", index, dataID, location)
	return nil
}

// RetrieveShard gets a shard from memory or disk if available
func (ims *InMemoryShardStore) RetrieveShard(dataID string, index int, location string) ([]byte, error) {
	ims.mu.RLock()
	defer ims.mu.RUnlock()

	// Try to get from memory first
	shards, exists := ims.ShardStore[dataID]
	if exists {
		shard, exists := shards[index]
		if exists {
			fmt.Printf("Retrieved shard %d for DataID: %s from memory\n", index, dataID)
			return shard, nil
		}
	}

	// If not in memory, try to load from disk
	shard, err := ims.readShardFromDisk(dataID, index, location)
	if err != nil {
		return nil, fmt.Errorf("no shards found for DataID: %s", dataID)
	}

	// Store in memory for future use
	if _, exists := ims.ShardStore[dataID]; !exists {
		ims.ShardStore[dataID] = make(map[int][]byte)
	}
	ims.ShardStore[dataID][index] = shard

	fmt.Printf("Retrieved shard %d for DataID: %s from location: %s\n", index, dataID, location)
	return shard, nil
}

// Helper functions for persistence

// getShardPath returns the path for a specific shard file
func (ims *InMemoryShardStore) getShardPath(dataID string, index int, location string) string {
	return filepath.Join(location, fmt.Sprintf("%s_%d.shard", dataID, index))
}

// writeShardToDisk writes a shard to disk
func (ims *InMemoryShardStore) writeShardToDisk(dataID string, index int, data []byte, location string) error {
	path := ims.getShardPath(dataID, index, location)
	return os.WriteFile(path, data, 0644)
}

// readShardFromDisk reads a shard from disk
func (ims *InMemoryShardStore) readShardFromDisk(dataID string, index int, location string) ([]byte, error) {
	path := ims.getShardPath(dataID, index, location)
	return os.ReadFile(path)
}
