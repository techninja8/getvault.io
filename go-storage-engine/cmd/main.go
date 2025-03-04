package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/techninja8/getvault.io/pkg/config"
	"github.com/techninja8/getvault.io/pkg/datastorage"
	"github.com/techninja8/getvault.io/pkg/sharding"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

func main() {
	// Load configuration.
	cfg := config.LoadConfig()

	// Initialize production logger.
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Choose a shard store backend.
	// For demonstration, we'll use the in-memory store.
	store := sharding.NewInMemoryShardStore()
	// To use the S3 store, uncomment below:
	// store := sharding.NewS3ShardStore(cfg.Bucket, cfg.S3Endpoint)

	app := &cli.App{
		Name:  "Vault Storage CLI",
		Usage: "Store and retrieve data using the Vault storage engine",
		Commands: []*cli.Command{
			{
				Name:    "store",
				Aliases: []string{"s"},
				Usage:   "Store data. Usage: store \"your data here\"",
				Action: func(c *cli.Context) error {
					data := []byte(c.Args().First())
					if len(data) == 0 {
						fmt.Println("Please provide data to store")
						return nil
					}
					dataID, err := datastorage.StoreData(data, store, cfg, logger)
					if err != nil {
						logger.Error("Store failed", zap.Error(err))
						return err
					}
					fmt.Printf("Data stored successfully with ID: %s\n", dataID)
					return nil
				},
			},
			{
				Name:    "retrieve",
				Aliases: []string{"r"},
				Usage:   "Retrieve data by DataID. Usage: retrieve <dataID>",
				Action: func(c *cli.Context) error {
					dataID := c.Args().First()
					if dataID == "" {
						fmt.Println("Please provide a DataID")
						return nil
					}
					data, err := datastorage.RetrieveData(dataID, store, cfg, logger)
					if err != nil {
						logger.Error("Retrieve failed", zap.Error(err))
						return err
					}
					fmt.Printf("Retrieved Data: %s\n", string(data))
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
