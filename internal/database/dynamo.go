package database

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoClient holds the DynamoDB service client.
type DynamoClient struct {
	svc *dynamodb.Client
}

// Product struct maps the Go structure to the DynamoDB item structure.
type Product struct {
	ID    string `dynamodbav:"ProductID"`
	Name  string `dynamodbav:"ProductName"`
	Price int    `dynamodbav:"Price"`
}

const tableName = "LocalProducts"
const localEndpoint = "http://localhost:8000"

// NewDynamoClient configures and returns a client connected to DynamoDB Local.
func NewDynamoClient(ctx context.Context) (*DynamoClient, error) {

	cfg, err := config.LoadDefaultConfig(ctx,
		// CRITICAL FIX 1: Set a Region
		config.WithRegion("us-west-2"), // <<< ADD THIS LINE

		// ... (existing EndpointResolverOptions code remains) ...
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == dynamodb.ServiceID {
					return aws.Endpoint{
						URL:           localEndpoint,
						SigningName:   "dynamodb",
						SigningRegion: "us-west-2",
					}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			},
		)),
		// CRITICAL FIX 2: Use static dummy credentials for signing
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(
				credentials.NewStaticCredentialsProvider("dummy", "dummy", ""),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load SDK configuration: %w", err)
	}

	svc := dynamodb.NewFromConfig(cfg)

	// Ensure table exists on startup (idempotent)
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
				AttributeName: aws.String("ProductID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("ProductID"),
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

// PutProduct stores a product item in the table.
func (c *DynamoClient) PutProduct(ctx context.Context, product Product) error {
	item, err := attributevalue.MarshalMap(product)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = c.svc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}

// GetProduct retrieves a product item by its ID.
func (c *DynamoClient) GetProduct(ctx context.Context, productID string) (Product, error) {
	result, err := c.svc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"ProductID": &types.AttributeValueMemberS{Value: productID},
		},
	})
	if err != nil {
		return Product{}, err
	}

	if result.Item == nil {
		return Product{}, fmt.Errorf("product ID %s not found", productID)
	}

	var product Product
	err = attributevalue.UnmarshalMap(result.Item, &product)
	if err != nil {
		return Product{}, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return product, nil
}
