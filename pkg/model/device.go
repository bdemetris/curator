package model

import "time"

// Device is the public data model used accross the app
type Device struct {
	AssetTag     string     `dynamodbav:"AssetTag"`
	DeviceType   string     `dynamodbav:"DeviceType"`
	DeviceMake   string     `dynamodbav:"DeviceMake"`
	DeviceModel  string     `dynamodbav:"DeviceModel"`
	Location     string     `dynamodbav:"Location"`
	AssignedTo   string     `dynamodbav:"AssignedTo"`
	AssignedDate *time.Time `dynamodbav:"AssignedDate"`
}
