package datastorage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/techninja8/getvault.io/pkg/config"
	"github.com/techninja8/getvault.io/pkg/encryption"
	"github.com/techninja8/getvault.io/pkg/erasurecoding"
	"github.com/techninja8/getvault.io/pkg/proofofinclusion"
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

func StorageLocationFileCreator() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 12)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return "strl_" + string(b) + ".config"
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

	// Log original data size for debugging
	logger.Info("Original data size before encryption", zap.Int("size", len(data)))

	// Check if the data starts with ZIP signature for debugging
	if len(data) >= 4 {
		logger.Info("Data header signature", zap.String("hex", fmt.Sprintf("%x", data[:4])))
		if string(data[:4]) != "PK\x03\x04" && strings.HasSuffix(filePath, ".zip") {
			logger.Warn("Expected ZIP file doesn't have proper signature")
		}
	}

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

	// Log encrypted data size for debugging
	logger.Info("Encrypted data size", zap.Int("size", len(cipherText)))

	dataID := GenerateDataID(cipherText)

	shards, err := erasurecoding.Encode(cipherText)
	if err != nil {
		logger.Error("Erasure coding failed", zap.Error(err))
		return "", err
	}

	// Log total shards size for debugging
	totalShardSize := 0
	for _, shard := range shards {
		totalShardSize += len(shard)
	}
	logger.Info("Total size of all shards", zap.Int("size", totalShardSize))

	// Store each shard.
	for idx, shard := range shards {
		location := locations[idx] // Use locations from the configuration file
		logger.Info("Storing shard", zap.Int("shard", idx), zap.String("location", location), zap.Int("size", len(shard)))
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
	dataToAppend += "storage_locations: {\n"
	for idx, location := range locations {
		dataToAppend += fmt.Sprintf("  shard_%d: %s\n", idx, location)
	}
	dataToAppend += "}\n"
	dataToAppend += "Proofs: {\n"
	tree, err := proofofinclusion.BuildMerkleTree(shards)
	if err != nil {
		log.Fatal("failed to build Merkle tree: %w", err)
	}
	for i, shard := range shards {
		if shard == nil {
			continue
		}
		proof, err := proofofinclusion.GetProof(tree, shard)
		if err != nil {
			log.Fatal("failed to get proof for shard")
		}
		proof_of_shard := fmt.Sprintf("Proof for shard %d: %s\n", i, proof)
		dataToAppend += "  " + proof_of_shard
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
			logger.Info("Retrieved shard", zap.Int("index", i), zap.String("location", location))
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

	// Debugging: Check the size of the reconstructed cipherText
	logger.Info("Reconstructed cipherText size", zap.Int("size", len(cipherText)))

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

	// Debugging: Check the size of the decrypted plainText
	logger.Info("Decrypted plainText size", zap.Int("size", len(plainText)))

	// Validate if this is a ZIP file by checking for ZIP signature (PK header)
	if len(plainText) >= 4 && string(plainText[:4]) != "PK\x03\x04" {
		logger.Warn("Retrieved data does not have a valid ZIP file signature",
			zap.String("signature", fmt.Sprintf("%x", plainText[:4])))
	}

	return plainText, nil
}

// VerifyData verifies the data availability using cryptographic proofs.
func VerifyData(metadatafile string, store sharding.ShardStore, logger *zap.Logger) error {
	dataID, err := MetadataFileReader(metadatafile, "dataID")
	if err != nil {
		return fmt.Errorf("error reading metadata file: %w", err)
	}

	// Read storage locations from the metadata file
	locations := make([]string, 14)
	for i := 0; i < 14; i++ {
		key := fmt.Sprintf("shard_%d", i)
		location, err := MetadataFileReader(metadatafile, key)
		if err != nil {
			return fmt.Errorf("error reading shard location from metadata file: %w", err)
		}
		locations[i] = location
	}

	// Retrieve shards from the storage locations
	shards := make([][]byte, len(locations))
	for i, location := range locations {
		shard, err := store.RetrieveShard(dataID, i, location)
		if err != nil {
			logger.Warn("Shard retrieval failed", zap.Int("index", i), zap.String("location", location), zap.Error(err))
			continue
		}
		shards[i] = shard
	}

	// Build Merkle Tree
	tree, err := proofofinclusion.BuildMerkleTree(shards)
	if err != nil {
		return fmt.Errorf("failed to build Merkle tree: %w", err)
	}

	// Read original proofs from metadata file
	proofs := make([]string, 14)
	for i := 0; i < 14; i++ {
		key := fmt.Sprintf("Proof for shard %d", i)
		proof, err := MetadataFileReader(metadatafile, key)
		if err != nil {
			return fmt.Errorf("failed to read proof from metadata file: %w", err)
		}
		proofs[i] = proof
	}

	// Generate and compare proof for each shard
	for i, shard := range shards {
		if shard == nil {
			continue
		}
		proof, err := proofofinclusion.GetProof(tree, shard)
		if err != nil {
			return fmt.Errorf("failed to get proof for shard %d: %w", i, err)
		}
		fmt.Printf("Shard_%d Verification: %t\n", i, proof == proofs[i])
	}

	return nil
}

// SetupStorage sets up the storage location configuration file.
func SetupStorage(locations []string, logger *zap.Logger) (string, error) {
	if len(locations) != 14 {
		return "", fmt.Errorf("storage locations incomplete, requires 14 locations")
	}

	for _, location := range locations {
		if location == "" {
			return "", fmt.Errorf("invalid storage location: %s", location)
		}
	}

	storageFile := StorageLocationFileCreator()
	file, err := os.OpenFile(storageFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create storage location configuration file: %w", err)
	}
	defer file.Close()

	for _, location := range locations {
		if _, err := file.WriteString(location + "\n"); err != nil {
			return "", fmt.Errorf("failed to write to storage location configuration file: %w", err)
		}
	}

	logger.Info("Storage location configuration file created successfully", zap.String("file", storageFile))
	return storageFile, nil
}
