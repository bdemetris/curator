package model

import "time"

// JiraAsset represents the core structure of an item retrieved from the Jira Assets API.
// Note: Fields are typically complex (like object references), but we simplify here.
type JiraAsset struct {
	Key          string    `json:"objectKey"` // The unique identifier, like I-12345
	ID           string    `json:"id"`        // The internal object ID
	Name         string    `json:"name"`
	ObjectSchema string    `json:"objectSchema"` // Name of the Schema (e.g., "IT Assets")
	ObjectType   string    `json:"objectType"`   // Name of the Type (e.g., "Laptop", "Server")
	Status       string    // Derived from attributes
	Owner        string    // Derived from attributes (e.g., Assigned User)
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"lastModified"`
}

// AssetSearchOptions defines parameters for searching Jira Assets.
type AssetSearchOptions struct {
	ObjectSchemaID string // The ID of the schema to search within
	AQL            string // The Jira Assets Query Language (AQL) string
	ResultsPerPage int
	StartAt        int
}
