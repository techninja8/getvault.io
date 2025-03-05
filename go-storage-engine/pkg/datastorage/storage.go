package datastorage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/techninja8/getvault.io/pkg/config"
	"github.com/techninja8/getvault.io/pkg/encryption"
	"github.com/techninja8/getvault.io/pkg/erasurecoding"
	"github.com/techninja8/getvault.io/pkg/sharding"
)

var (
	errMissingKey       = errors.New("encryption key not set in configuration")
	errInvalidKeyLength = errors.New("invalid encryption key length; must be 32 bytes for AES-256")
	errInvalidLocations = errors.New("invalid storage location configuration file; must contain 14 locations")
)

// GetEncryptionKey converts the configuration key from hex.
func GetEncryptionKey(cfg *config.Config) ([]byte, error) {
	key, err := hex.DecodeString(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errInvalidKeyLength
	}
	return key, nil
}

// GenerateEncryptionKey creates a new random encryption key.
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
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

func MetadataFileReader(filename string, key string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ": ", 2)
		// If not key value, pls continue
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if k == key {
			return v, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return "", errors.New("key not found in metadata file")
}

func MetadataFileCreator() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 12)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return "vault_session_" + string(b) + ".vmd"
}

// ReadStorageLocations reads storage locations from a configuration file.
func ReadStorageLocations(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening storage location configuration file: %w", err)
	}
	defer file.Close()

	var locations []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		location := strings.TrimSpace(scanner.Text())
		if location != "" {
			locations = append(locations, location)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read storage location configuration file: %w", err)
	}

	if len(locations) != 14 {
		return nil, errInvalidLocations
	}

	return locations, nil
}

// StoreData encrypts data, applies erasure coding, and stores each shard.
func StoreData(data []byte, store sharding.ShardStore, cfg *config.Config, locations []string, logger *zap.Logger, filePath string) (string, error) {
	newmetadatafile := MetadataFileCreator()

	key, err := GetEncryptionKey(cfg)
	if err != nil {
		logger.Error("Failed to get encryption key", zap.Error(err))
		return "", err
	}

	cipherText, err := encryption.Encrypt(data, key)
	if err != nil {
		logger.Error("Encryption failed", zap.Error(err))
		return "", err
	}

	dataID := GenerateDataID(cipherText)

	shards, err := erasurecoding.Encode(cipherText)
	if err != nil {
		logger.Error("Erasure coding failed", zap.Error(err))
		return "", err
	}

	// Store each shard.
	for idx, shard := range shards {
		location := locations[idx] // Use locations from the configuration file
		logger.Info("Storing shard", zap.Int("shard", idx), zap.String("location", location))
		if err := store.StoreShard(dataID, idx, shard, location); err != nil {
			logger.Error("Storing shard failed", zap.Int("shard", idx), zap.String("location", location), zap.Error(err))
			return "", err
		}
	}

	// Extract filename and format
	filename := filepath.Base(filePath)
	format := strings.TrimPrefix(filepath.Ext(filePath), ".")

	// Update metadata file with new fields
	logger.Info("Updating metadata file", zap.String("metadataFile", newmetadatafile))
	dataToAppend := fmt.Sprintf("dataID: %s\nfilename: %s\nfilesize: %d\nformat: %s\ncreation_date: %s\n", dataID, filename, len(data), format, time.Now().Format(time.RFC3339))
	dataToAppend += "storage_locations: "
	dataToAppend += "{\n"
	for idx, location := range locations {
		dataToAppend += fmt.Sprintf("  shard_%d: %s\n", idx, location)
	}
	dataToAppend += "}\n"

	file, err := os.OpenFile(newmetadatafile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("couldn't open or create a new metadata file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(dataToAppend); err != nil {
		return "", fmt.Errorf("couldn't update metadata content: %w", err)
	}

	logger.Info("Data stored successfully", zap.String("dataID", dataID))
	return dataID, nil
}

// RetrieveData assembles shards, decodes, and decrypts the data.
// Tolerates missing shards within parity limits.
func RetrieveData(metadatafile string, store sharding.ShardStore, cfg *config.Config, logger *zap.Logger) ([]byte, error) {
	metakey := "dataID"
	dataID, err := MetadataFileReader(metadatafile, metakey)
	if err != nil {
		return nil, fmt.Errorf("error reading metadata file: %w", err)
	}

	// Read storage locations from the metadata file
	locations := make([]string, 14)
	for i := 0; i < 14; i++ {
		key := fmt.Sprintf("shard_%d", i)
		location, err := MetadataFileReader(metadatafile, key)
		if err != nil {
			return nil, fmt.Errorf("error reading shard location from metadata file: %w", err)
		}
		locations[i] = location
	}

	totalShards := erasurecoding.DataShards + erasurecoding.ParityShards
	shards := make([][]byte, totalShards)
	missing := 0
	for i := 0; i < totalShards; i++ {
		location := locations[i] // Retrieve from respective locations
		shard, err := store.RetrieveShard(dataID, i, location)
		if err != nil {
			logger.Warn("Shard retrieval failed", zap.Int("index", i), zap.String("location", location), zap.Error(err))
			shards[i] = nil
			missing++
		} else {
			shards[i] = shard
		}
	}
	if missing > erasurecoding.ParityShards {
		return nil, errors.New("insufficient shards for reconstruction")
	}

	cipherText, err := erasurecoding.Decode(shards)
	if err != nil {
		logger.Error("Erasure decoding failed", zap.Error(err))
		return nil, err
	}

	key, err := GetEncryptionKey(cfg)
	if err != nil {
		logger.Error("Failed to get encryption key", zap.Error(err))
		return nil, err
	}

	plainText, err := encryption.Decrypt(cipherText, key)
	if err != nil {
		logger.Error("Decryption failed", zap.Error(err))
		return nil, err
	}
	return plainText, nil
}
