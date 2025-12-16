package store

import (
	"bdemetris/curator/pkg/model"
	"context"
)

// Store defines the methods for interacting with the database.
// All application logic should depend only on this interface.
type Store interface {
	Close() error

	// Device Operations
	PutDevice(ctx context.Context, device model.Device) error
	GetDevice(ctx context.Context, deviceID string) (model.Device, error)
	ListDevices(ctx context.Context) ([]model.Device, error)
	UpdateDevice(ctx context.Context, deviceID string, updates map[string]interface{}) error
}
