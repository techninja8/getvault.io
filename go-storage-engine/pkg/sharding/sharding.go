package sharding

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type ShardStore interface {
	StoreShard(dataID string, index int, shard []byte) error
	RetrieveShard(dataID string, index int) ([]byte, error)
}

// InMemoryShardStore with file persistence
type InMemoryShardStore struct {
	ShardStore map[string]map[int][]byte
	mu         sync.RWMutex
	dataPath   string // Path to save/load data
}

func NewInMemoryShardStore() *InMemoryShardStore {
	// Set path to store data - using current directory with a "shards" subfolder
	dataPath := filepath.Join(".", "shards")

	// Create directory if it doesn't exist
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			fmt.Printf("Warning: Could not create data directory: %v\n", err)
		}
	}

	store := &InMemoryShardStore{
		ShardStore: make(map[string]map[int][]byte),
		dataPath:   dataPath,
	}

	// Load any existing data
	store.loadFromDisk()

	return store
}

// StoreShard stores a shard and persists it to disk
func (ims *InMemoryShardStore) StoreShard(dataID string, index int, shard []byte) error {
	ims.mu.Lock()
	defer ims.mu.Unlock()

	// Store in memory
	if _, exists := ims.ShardStore[dataID]; !exists {
		ims.ShardStore[dataID] = make(map[int][]byte)
	}
	ims.ShardStore[dataID][index] = shard

	// Store to disk
	if err := ims.writeShardToDisk(dataID, index, shard); err != nil {
		return fmt.Errorf("failed to persist shard: %w", err)
	}

	fmt.Printf("Stored shard %d for DataID: %s\n", index, dataID)
	return nil
}

// RetrieveShard gets a shard from memory or disk if available
func (ims *InMemoryShardStore) RetrieveShard(dataID string, index int) ([]byte, error) {
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
	shard, err := ims.readShardFromDisk(dataID, index)
	if err != nil {
		return nil, fmt.Errorf("no shards found for DataID: %s", dataID)
	}

	// Store in memory for future use
	if _, exists := ims.ShardStore[dataID]; !exists {
		ims.ShardStore[dataID] = make(map[int][]byte)
	}
	ims.ShardStore[dataID][index] = shard

	fmt.Printf("Retrieved shard %d for DataID: %s from disk\n", index, dataID)
	return shard, nil
}

// Helper functions for persistence

// getShardPath returns the path for a specific shard file
func (ims *InMemoryShardStore) getShardPath(dataID string, index int) string {
	return filepath.Join(ims.dataPath, fmt.Sprintf("%s_%d.shard", dataID, index))
}

// writeShardToDisk writes a shard to disk
func (ims *InMemoryShardStore) writeShardToDisk(dataID string, index int, data []byte) error {
	path := ims.getShardPath(dataID, index)
	return ioutil.WriteFile(path, data, 0644)
}

// readShardFromDisk reads a shard from disk
func (ims *InMemoryShardStore) readShardFromDisk(dataID string, index int) ([]byte, error) {
	path := ims.getShardPath(dataID, index)
	return ioutil.ReadFile(path)
}

// loadFromDisk loads all shards from disk
func (ims *InMemoryShardStore) loadFromDisk() {
	files, err := ioutil.ReadDir(ims.dataPath)
	if err != nil {
		fmt.Printf("Warning: Could not read shards directory: %v\n", err)
		return
	}

	// Iterate through all files in the directory
	loaded := 0
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".shard" {
			continue
		}

		// Parse dataID and index from filename
		var dataID string
		var index int
		_, err := fmt.Sscanf(file.Name(), "%s_%d.shard", &dataID, &index)
		if err != nil {
			continue
		}

		// Read the shard data
		path := filepath.Join(ims.dataPath, file.Name())
		data, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}

		// Store in memory
		if _, exists := ims.ShardStore[dataID]; !exists {
			ims.ShardStore[dataID] = make(map[int][]byte)
		}
		ims.ShardStore[dataID][index] = data
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("Loaded %d shards from disk\n", loaded)
	}
}
