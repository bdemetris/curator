package model

import "time"

// Device is the public data model used accross the app
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
