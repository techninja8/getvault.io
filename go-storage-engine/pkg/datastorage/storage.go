package datastorage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"math/rand"
	"os"

	"github.com/techninja8/getvault.io/pkg/encryption"
	"github.com/techninja8/getvault.io/pkg/erasurecoding"
	shardstore "github.com/techninja8/getvault.io/pkg/sharding"
)

const (
	keyEnvVar = "ENCRYPTION_KEY"
	keyLength = 32 // AES-256 requires a 32-byte key
)

// GetEncryptionKey retrieves the encryption key from an environment variable.
func GetEncryptionKey() ([]byte, error) {
	keyHex := os.Getenv(keyEnvVar)
	if keyHex == "" {
		return nil, errMissingKey
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, err
	}
	if len(key) != keyLength {
		return nil, errInvalidKeyLength
	}
	return key, nil
}

var (
	errMissingKey       = errors.New("encryption key not set in environment variable")
	errInvalidKeyLength = errors.New("invalid encryption key length; must be 32 bytes for AES-256")
)

// GenerateEncryptionKey creates a new random encryption key.
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, keyLength)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// GenerateDataID creates a unique identifier using SHA-256.
func GenerateDataID(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// StoreData encrypts the data, applies erasure coding, and stores each shard.
func StoreData(data []byte, store shardstore.ShardStore) (string, error) {
	key, err := GetEncryptionKey()
	if err != nil {
		log.Println("Failed to get encryption key:", err)
		return "", err
	}

	cipherText, err := encryption.Encrypt(data, key)
	if err != nil {
		return "", err
	}

	dataID := GenerateDataID(cipherText)

	shards, err := erasurecoding.Encode(cipherText)
	if err != nil {
		return "", err
	}

	// Store each shard using the provided shard storage.
	for idx, shard := range shards {
		if err := store.StoreShard(dataID, idx, shard); err != nil {
			return "", err
		}
	}

	log.Printf("Data stored with %d shards. DataID: %s\n", len(shards), dataID)
	return dataID, nil
}

// RetrieveData assembles shards, decodes the data, and decrypts it.
func RetrieveData(dataID string, store shardstore.ShardStore) ([]byte, error) {
	// Total shards = dataShards + parityShards, in this example 6.
	const totalShards = 6
	shards := make([][]byte, totalShards)
	for i := 0; i < totalShards; i++ {
		shard, err := store.RetrieveShard(dataID, i)
		if err != nil {
			// In a production system, you may tolerate missing shards if within parity limits.
			log.Printf("Warning: unable to retrieve shard %d: %v\n", i, err)
			shards[i] = nil
		} else {
			shards[i] = shard
		}
	}

	cipherText, err := erasurecoding.Decode(shards)
	if err != nil {
		return nil, err
	}

	key, err := GetEncryptionKey()
	if err != nil {
		return nil, err
	}

	plainText, err := encryption.Decrypt(cipherText, key)
	if err != nil {
		return nil, err
	}
	return plainText, nil
}
