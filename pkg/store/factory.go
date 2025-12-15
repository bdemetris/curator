package store

import (
	"context"
	"fmt"
)

// DatabaseProvider constants define the available database options.
const (
	ProviderDynamoDB = "dynamodb"
	ProviderPostgres = "postgres"
	ProviderMemory   = "in-memory"
)

type StoreConstructor func(ctx context.Context, config string) (Store, error)

// StoreConfig holds all necessary configuration strings for the database.
type StoreConfig struct {
	Provider         string
	DynamoDBEndpoint string // e.g., "http://localhost:8000" or empty for AWS
	PostgresDSN      string // e.g., "user=... password=..."
}

func NewStoreFactory(ctx context.Context, cfg StoreConfig, constructors map[string]StoreConstructor) (Store, error) {
	constructor, ok := constructors[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported database provider '%s'. Available: %v", cfg.Provider, constructors)
	}

	// Determine which config string to pass based on the provider
	var configString string
	switch cfg.Provider {
	case ProviderDynamoDB:
		configString = cfg.DynamoDBEndpoint
	case ProviderPostgres:
		configString = cfg.PostgresDSN
	}

	// Call the external constructor function
	return constructor(ctx, configString)
}
