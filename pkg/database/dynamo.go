package database

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"bdemetris/curator/pkg/model"
	"bdemetris/curator/pkg/store"
)

type DynamoClient struct {
	svc *dynamodb.Client
}

var _ store.Store = (*DynamoClient)(nil)

const tableName = "Devices"

// NewDynamoClient configures and returns a client connected to DynamoDB Local.
func NewDynamoStore(ctx context.Context, endpoint string) (store.Store, error) {
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
			dynamodb.EndpointResolverFromURL(endpoint),
		),

		func(o *dynamodb.Options) {
			o.Region = "us-west-2"
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
				AttributeName: aws.String("AssetTag"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("AssetTag"),
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

// Close is implemented to satisfy the Store interface.
// Since the AWS SDK client doesn't need explicit closing, we return nil.
func (c *DynamoClient) Close() error {
	log.Println("DynamoDB client does not require explicit closing.")
	return nil
}

// PutDevice stores a Device item in the table.
func (c *DynamoClient) PutDevice(ctx context.Context, device model.Device) error {
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
func (c *DynamoClient) GetDevice(ctx context.Context, deviceID string) (model.Device, error) {
	result, err := c.svc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"AssetTag": &types.AttributeValueMemberS{Value: deviceID},
		},
	})
	if err != nil {
		return model.Device{}, err
	}

	if result.Item == nil {
		return model.Device{}, fmt.Errorf("AssetTag %s not found", deviceID)
	}

	var device model.Device
	err = attributevalue.UnmarshalMap(result.Item, &device)
	if err != nil {
		return model.Device{}, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return device, nil
}

// ListDevices retrieves all device items from the DynamoDB table.
func (c *DynamoClient) ListDevices(ctx context.Context) ([]model.Device, error) {
	// A Scan without a FilterExpression retrieves all items
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Select:    types.SelectAllAttributes,
	}

	result, err := c.svc.Scan(ctx, scanInput)
	if err != nil {
		return nil, fmt.Errorf("dynamodb scan failed: %w", err)
	}

	var devices []model.Device
	err = attributevalue.UnmarshalListOfMaps(result.Items, &devices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal devices: %w", err)
	}

	return devices, nil
}

func (c *DynamoClient) UpdateDevice(ctx context.Context, deviceID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return fmt.Errorf("no update parameters provided for device ID %s", deviceID)
	}

	updateExpressionParts := []string{}
	attributeNames := map[string]string{}
	attributeValues := map[string]types.AttributeValue{}

	i := 0
	for key, value := range updates {
		namePlaceholder := fmt.Sprintf("#a%d", i)
		valuePlaceholder := fmt.Sprintf(":v%d", i)

		av, err := attributevalue.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal update value for %s: %w", key, err)
		}

		updateExpressionParts = append(updateExpressionParts, fmt.Sprintf("%s = %s", namePlaceholder, valuePlaceholder))
		attributeNames[namePlaceholder] = key
		attributeValues[valuePlaceholder] = av
		i++
	}

	updateExpression := "SET " + strings.Join(updateExpressionParts, ", ")

	// --- 🛠️ THE CRITICAL FIX IS HERE ---
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			// MUST be "SerialNumber" to match your CreateTable schema
			"AssetTag": &types.AttributeValueMemberS{Value: deviceID},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeNames:  attributeNames,
		ExpressionAttributeValues: attributeValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	}

	_, err := c.svc.UpdateItem(ctx, updateInput)
	if err != nil {
		return fmt.Errorf("dynamodb update failed for ID %s: %w", deviceID, err)
	}

	return nil
}
