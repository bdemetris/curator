package store

import (
	"context"
	"fmt"
)

// DatabaseProvider constants define the available database options.
const (
	ProviderDynamoDB   = "dynamodb"
	ProviderJiraAssets = "jira-api"
	ProviderMemory     = "in-memory"
)

type StoreConstructor func(ctx context.Context, cfg StoreConfig) (Store, error)

// StoreConfig holds all necessary configuration strings for the database.
type StoreConfig struct {
	Provider         string
	DynamoDBEndpoint string // e.g., "http://localhost:8000" or empty for AWS
	JiraToken        string // e.g., "user=... password=..."
	JiraBaseURL      string
	JiraEmail        string
}

func NewStoreFactory(ctx context.Context, cfg StoreConfig, constructors map[string]StoreConstructor) (Store, error) {
	constructor, ok := constructors[cfg.Provider]
	if !ok {
		availableProviders := make([]string, 0, len(constructors))
		for key := range constructors {
			availableProviders = append(availableProviders, key)
		}
		return nil, fmt.Errorf("unsupported database provider '%s'. Available: %v", cfg.Provider, availableProviders)
	}

	return constructor(ctx, cfg)
}
