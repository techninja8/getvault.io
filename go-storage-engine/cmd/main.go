// Entry Point for Vault's storage engine service

package main

import (
	"github.com/techninja8/getvault.io/datastorage"
	"log"
)

func main() {
	// Sample data to store
	data := []byte("Hello, Vault Storage!")

	// Store data and obtain unique DataID
	dataID, storeErr := datastorage.storeData(data)
	if storeErr != nil {
		log.Fatalf("Failed to store data: %v", storeErr)
	}
	log.Printf("Data stored with ID: %s", &dataID)

	retrievedData, retrieveErr := datastorage.retrievedData(dataID)
	if retrieveErr != nil {
		log.Fatalf("Failed to retrieve data %v", retrieveErr)
	}
	log.Printf("Retrieved Data: %s", string(retrievedData))
}
