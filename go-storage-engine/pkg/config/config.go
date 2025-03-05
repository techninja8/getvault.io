package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	EncryptionKey         string
	DataShards            int
	ParityShards          int
	S3Endpoint            string
	Bucket                string
	MetricsInterval       time.Duration
	ShardStorageLocations []string
}

func LoadConfig() *Config {
	viper.AutomaticEnv()
	// Set defaults
	viper.SetDefault("DATA_SHARDS", 8)
	viper.SetDefault("PARITY_SHARDS", 6)
	viper.SetDefault("S3_ENDPOINT", "https://s3.amazonaws.com")
	viper.SetDefault("BUCKET", "your-bucket")
	viper.SetDefault("METRICS_INTERVAL", 10*time.Second)
	viper.SetDefault("SHARD_STORAGE_LOCATIONS", []string{"/path/to/location1", "/path/to/location2"}) // Default storage locations

	cfg := &Config{
		EncryptionKey:         viper.GetString("ENCRYPTION_KEY"),
		DataShards:            viper.GetInt("DATA_SHARDS"),
		ParityShards:          viper.GetInt("PARITY_SHARDS"),
		S3Endpoint:            viper.GetString("S3_ENDPOINT"),
		Bucket:                viper.GetString("BUCKET"),
		MetricsInterval:       viper.GetDuration("METRICS_INTERVAL"),
		ShardStorageLocations: viper.GetStringSlice("SHARD_STORAGE_LOCATIONS"), // We'll use this to load storage locations
	}

	if cfg.EncryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY must be set")
	}

	if len(cfg.ShardStorageLocations) == 0 {
		log.Fatal("SHARD_STORAGE_LOCATIONS must be set")
	}

	return cfg
}
