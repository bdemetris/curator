package store

import (
	"context"
	"time"
)

// Device struct maps the Go structure to the DynamoDB item structure.
// NOTE: This should match the struct in your database package.
type Device struct {
	ID           string    `dynamodbav:"ObjectId"`
	SerialNumber string    `dynamodbav:"SerialNumber"`
	AssetTag     int       `dynamodbav:"AssetTag"`
	AssignedTo   string    `dynamodbav:"AssignedTo"`
	AssignedDate time.Time `dynamodbav:"AssignedDate"`
	Manufacturer string    `dynamodbav:"Manufacturer"`
	ModelName    string    `dynamodbav:"ModelName"`
	DeviceType   string    `dynamodbav:"DeviceType"`
	Location     string    `dynamodbav:"Location"`
}

// Store defines the methods for interacting with the database.
// All application logic should depend only on this interface.
type Store interface {
	// Close handles closing the underlying resource (though DynamoDB clients usually don't need this,
	// it's good practice for general 'Store' contracts).
	Close() error

	// Device Operations
	PutDevice(ctx context.Context, device Device) error
	GetDevice(ctx context.Context, deviceID string) (Device, error)
	ListDevices(ctx context.Context, deviceType string) ([]Device, error)
	UpdateDevice(ctx context.Context, deviceID string, updates map[string]interface{}) error
	// DeleteDevice(ctx context.Context, deviceID string) error // Dont implement Delete
}
