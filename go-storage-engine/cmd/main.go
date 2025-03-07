package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/techninja8/getvault.io/pkg/config"
	"github.com/techninja8/getvault.io/pkg/datastorage"
	"github.com/techninja8/getvault.io/pkg/proofofinclusion"
	"github.com/techninja8/getvault.io/pkg/sharding"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.LoadConfig()
	store := sharding.NewInMemoryShardStore()

	app := &cli.App{
		Name:  "vault",
		Usage: "Distributed Storage and Retrieval of Erasure-coded Data Shards Using Vault's Storage Engine",
		Commands: []*cli.Command{
			{
				Name:    "store",
				Aliases: []string{"s"},
				Usage:   "Store data. Usage: store <filename_or_directory> <storage-location-configuration>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 2 {
						return fmt.Errorf("please provide a file or directory to store and a storage location configuration file")
					}
					path := c.Args().Get(0)
					storageConfigPath := c.Args().Get(1)

					locations, err := datastorage.ReadStorageLocations(storageConfigPath)
					if err != nil {
						return fmt.Errorf("failed to read storage location configuration file: %w", err)
					}

					// Determine if the path is a directory or a file
					info, err := os.Stat(path)
					if err != nil {
						return fmt.Errorf("failed to stat path: %w", err)
					}

					var filePath string
					// In the store command action
					if info.IsDir() {
						// Zip the directory
						zipFilePath := filepath.Join(os.TempDir(), filepath.Base(path)+".zip")
						//logger.Info("Zipping directory",
						//	zap.String("source", path),
						//	zap.String("target", zipFilePath))

						err = datastorage.ZipDirectory(path, zipFilePath)
						if err != nil {
							return fmt.Errorf("failed to zip directory: %w", err)
						}

						// Verify the zip file before storing (Debugging)
						/* zipFileInfo, err := os.Stat(zipFilePath)
						if err != nil {
							return fmt.Errorf("failed to stat zip file: %w", err)
						}
						logger.Info("Original zip file size", zap.Int64("size", zipFileInfo.Size())) */

						// Validate the zip format
						isValid, err := datastorage.IsValidZipFile(zipFilePath)
						if err != nil || !isValid {
							logger.Error("Created ZIP file validation failed", zap.Error(err))
							return fmt.Errorf("created zip file is not valid: %w", err)
						}

						// Check zip contents by listing files in the archive
						zipReader, err := zip.OpenReader(zipFilePath)
						if err != nil {
							return fmt.Errorf("failed to open created zip: %w", err)
						}

						//logger.Info("ZIP archive contains", zap.Int("files", len(zipReader.File)))
						/* for i, f := range zipReader.File {
							logger.Info("ZIP entry",
								zap.Int("index", i),
								zap.String("name", f.Name),
								zap.Int64("size", int64(f.UncompressedSize64)))
						} */
						zipReader.Close()

						filePath = zipFilePath
					} else {
						filePath = path
					}

					data, err := os.ReadFile(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file: %w", err)
					}

					err = datastorage.Retry(3, 2*time.Second, logger, func() error {
						dataID, err := datastorage.StoreData(data, store, cfg, locations, logger, filePath)
						if err != nil {
							logger.Error("Store failed", zap.Error(err))
							return fmt.Errorf("store failed: %w", err)
						}
						fmt.Printf("Data stored with ID: %s\n", dataID)
						return nil
					})
					if err != nil {
						return fmt.Errorf("failed to store data after retries: %w", err)
					}
					return nil
				},
			},
			{
				Name:    "retrieve",
				Aliases: []string{"r"},
				Usage:   "Retrieve Data From Metadata File. Usage: retrieve <metadatafile>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("please provide a metadata file")
					}
					metadataFile := c.Args().Get(0)

					var data []byte
					err := datastorage.Retry(3, 2*time.Second, logger, func() error {
						retrievedData, err := datastorage.RetrieveData(metadataFile, store, cfg, logger)
						if err != nil {
							logger.Error("Retrieve failed", zap.Error(err))
							return fmt.Errorf("retrieve failed: %w", err)
						}
						data = retrievedData
						return nil
					})
					if err != nil {
						return fmt.Errorf("failed to retrieve data after retries: %w", err)
					}

					// Read filename from metadata file
					filename, err := datastorage.MetadataFileReader(metadataFile, "filename")
					if err != nil {
						return fmt.Errorf("failed to read filename from metadata file: %w", err)
					}

					// Debugging: Check the size of the retrieved data
					//logger.Info("Retrieved data size", zap.Int("size", len(data)))

					// Check if we expect a ZIP file
					isZipFile := strings.HasSuffix(filename, ".zip")

					// If expecting a ZIP, validate the file signature first
					if isZipFile && (len(data) < 4 || string(data[:4]) != "PK\x03\x04") {
						logger.Warn("Expected ZIP file but data does not have ZIP signature",
							zap.String("expected_signature", "504B0304"),
							zap.String("actual_signature", fmt.Sprintf("%x", data[:min(4, len(data))])))
					}

					if err := os.WriteFile(filename, data, 0644); err != nil {
						return fmt.Errorf("failed to write retrieved data: %w", err)
					}
					fmt.Printf("Data retrieved and saved to: %s\n", filename)

					// Determine if the retrieved file is a zip file and extract if so
					if isZipFile {
						extractDir := strings.TrimSuffix(filename, ".zip")

						// Verify the file is a valid ZIP before attempting to extract
						_, err := zip.OpenReader(filename)
						if err != nil {
							logger.Error("Retrieved file is not a valid ZIP", zap.Error(err))
							return fmt.Errorf("failed to process ZIP file: %w", err)
						}

						err = datastorage.Unzip(filename, extractDir)
						if err != nil {
							logger.Error("Failed to unzip file", zap.Error(err))
							return fmt.Errorf("failed to unzip file: %w", err)
						}

						fmt.Printf("Data extracted to: %s\n", extractDir)
						err = os.Remove(filename)
						if err != nil {
							logger.Error("Failed to remove zip file", zap.Error(err))
						}
					}

					return nil
				},
			},
			{
				Name:    "verify",
				Aliases: []string{"v"},
				Usage:   "Verify data availability using cryptographic proofs. Usage: verify <metadatafile>",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("please provide a metadata file")
					}
					metadataFile := c.Args().Get(0)

					err := datastorage.Retry(3, 2*time.Second, logger, func() error {
						// Read dataID from metadata file
						dataID, err := datastorage.MetadataFileReader(metadataFile, "dataID")
						if err != nil {
							return fmt.Errorf("failed to read dataID from metadata file: %w", err)
						}

						// Read storage locations from metadata file
						locations := make([]string, 14)
						for i := 0; i < 14; i++ {
							key := fmt.Sprintf("shard_%d", i)
							location, err := datastorage.MetadataFileReader(metadataFile, key)
							if err != nil {
								return fmt.Errorf("failed to read shard location from metadata file: %w", err)
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

						// Generate and print proof for each shard
						for i, shard := range shards {
							if shard == nil {
								continue
							}
							proof, err := proofofinclusion.GetProof(tree, shard)
							if err != nil {
								return fmt.Errorf("failed to get proof for shard %d: %w", i, err)
							}
							fmt.Printf("Proof for shard %d: %s\n", i, proof)
						}

						return nil
					})
					if err != nil {
						return fmt.Errorf("failed to verify data after retries: %w", err)
					}

					return nil
				},
			},
			{
				Name:    "exit",
				Aliases: []string{"x"},
				Usage:   "Exit the CLI",
				Action: func(c *cli.Context) error {
					reader := bufio.NewReader(os.Stdin)
					fmt.Print("Are you sure you want to exit? (y/n): ")
					resp, _ := reader.ReadString('\n')
					resp = strings.TrimSpace(strings.ToLower(resp))
					if resp == "y" || resp == "yes" {
						fmt.Println("Exiting CLI...")
						os.Exit(0)
					}
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal("CLI failed", zap.Error(err))
	}
}
