package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const tableName = "LocalDevices"
const localEndpoint = "http://localhost:8000"

// DynamoClient holds the DynamoDB service client.
type DynamoClient struct {
	svc *dynamodb.Client
}

// Device struct maps the Go structure to the DynamoDB item structure.
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

// NewDynamoClient configures and returns a client connected to DynamoDB Local.
func NewDynamoClient(ctx context.Context) (*DynamoClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-west-2"),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(
				credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load SDK configuration: %w", err)
	}

	svc := dynamodb.NewFromConfig(cfg,
		dynamodb.WithEndpointResolver(
			dynamodb.EndpointResolverFromURL(localEndpoint),
		),

		func(o *dynamodb.Options) {
			o.Region = "us-west-2"
			o.ClientLogMode = aws.LogSigning
		},
	)

	if err := ensureTableExists(ctx, svc); err != nil {
		return nil, fmt.Errorf("failed to ensure table exists: %w", err)
	}

	return &DynamoClient{svc: svc}, nil
}

// ensureTableExists checks for and creates the required table.
func ensureTableExists(ctx context.Context, svc *dynamodb.Client) error {
	_, err := svc.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tableName)})
	if err == nil {
		fmt.Println("DynamoDB table already exists. Skipping creation.")
		return nil
	}

	log.Printf("Creating DynamoDB table: %s...", tableName)
	_, err = svc.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("SerialNumber"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("SerialNumber"),
				KeyType:       types.KeyTypeHash,
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	})
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}
	log.Println("Table created successfully.")
	return nil
}

// PutDevice stores a Device item in the table.
func (c *DynamoClient) PutDevice(ctx context.Context, device Device) error {
	item, err := attributevalue.MarshalMap(device)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = c.svc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}

// GetDevice retrieves a Device item by its Serial Number.
func (c *DynamoClient) GetDevice(ctx context.Context, deviceID string) (Device, error) {
	result, err := c.svc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"SerialNumber": &types.AttributeValueMemberS{Value: deviceID},
		},
	})
	if err != nil {
		return Device{}, err
	}

	if result.Item == nil {
		return Device{}, fmt.Errorf("SerialNumber %s not found", deviceID)
	}

	var device Device
	err = attributevalue.UnmarshalMap(result.Item, &device)
	if err != nil {
		return Device{}, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return device, nil
}
