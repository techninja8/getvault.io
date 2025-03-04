package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"github.com/techninja8/getvault.io/pkg/config"
	"github.com/techninja8/getvault.io/pkg/datastorage"
	"github.com/techninja8/getvault.io/pkg/sharding"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.LoadConfig()
	store := sharding.NewInMemoryShardStore()

	app := &cli.App{
		Name:  "vault",
		Usage: "A CLI for storing and retrieving encrypted, erasure-coded data",
		Commands: []*cli.Command{
			{
				Name:  "store",
				Usage: "Store a file",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("please provide a file to store")
					}
					filePath := c.Args().Get(0)
					data, err := os.ReadFile(filePath)
					if err != nil {
						return fmt.Errorf("failed to read file: %w", err)
					}
					dataID, err := datastorage.StoreData(data, store, cfg, logger)
					if err != nil {
						logger.Error("Store failed", zap.Error(err))
						return fmt.Errorf("store failed: %w", err)
					}
					fmt.Printf("Data stored with ID: %s\n", dataID)
					return nil
				},
			},
			{
				Name:  "retrieve",
				Usage: "Retrieve data using a metadata file",
				Action: func(c *cli.Context) error {
					if c.NArg() < 1 {
						return fmt.Errorf("please provide a metadata file")
					}
					metadataFile := c.Args().Get(0)
					data, err := datastorage.RetrieveData(metadataFile, store, cfg, logger)
					if err != nil {
						logger.Error("Retrieve failed", zap.Error(err))
						return fmt.Errorf("retrieve failed: %w", err)
					}
					outputFile := "retrieved_data.txt"
					if err := os.WriteFile(outputFile, data, 0644); err != nil {
						return fmt.Errorf("failed to write retrieved data: %w", err)
					}
					fmt.Printf("Data retrieved and saved to: %s\n", outputFile)
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal("CLI failed", zap.Error(err))
	}
}
