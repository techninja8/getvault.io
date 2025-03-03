package main

import (
	"log"

	"github.com/techninja8/getvault.io/pkg/datastorage"
	shardstore "github.com/techninja8/getvault.io/pkg/sharding"
)

func main() {
	// Example data to store.
	data := []byte("Hello, Vault Storage!")

	// Initialize the shard storage backend.
	store := shardstore.NewInMemoryShardStore()

	// Store the data.
	dataID, err := datastorage.StoreData(data, store)
	if err != nil {
		log.Fatalf("Failed to store data: %v", err)
	}
	log.Printf("Data stored with ID: %s", dataID)

	// Retrieve the data.
	retrievedData, err := datastorage.RetrieveData(dataID, store)
	if err != nil {
		log.Fatalf("Failed to retrieve data: %v", err)
	}
	log.Printf("Retrieved Data: %s", string(retrievedData))
}
