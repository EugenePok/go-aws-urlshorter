package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoClientConfig struct {
	Region   string
	Endpoint string
}

func NewDynamoClient(ctx context.Context, cfg DynamoClientConfig) (*dynamodb.Client, error) {
	loadOpts := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.Endpoint != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}), nil
}

type dynamoItem struct {
	Code      string `dynamodbav:"code"`
	LongURL   string `dynamodbav:"long_url"`
	CreatedAt string `dynamodbav:"created_at"`
}

type Dynamo struct {
	client *dynamodb.Client
	table  string
}

func NewDynamo(client *dynamodb.Client, table string) *Dynamo {
	return &Dynamo{client: client, table: table}
}

func (d *Dynamo) Save(ctx context.Context, link Link) error {
	item, err := attributevalue.MarshalMap(dynamoItem{
		Code:      link.Code,
		LongURL:   link.LongURL,
		CreatedAt: link.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("marshal item: %w", err)
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(d.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(code)"),
	})
	if err != nil {
		var failed *types.ConditionalCheckFailedException
		if errors.As(err, &failed) {
			return ErrCodeExists
		}
		return fmt.Errorf("put item: %w", err)
	}
	return nil
}

func (d *Dynamo) Get(ctx context.Context, code string) (Link, error) {
	out, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.table),
		Key: map[string]types.AttributeValue{
			"code": &types.AttributeValueMemberS{Value: code},
		},
	})
	if err != nil {
		return Link{}, fmt.Errorf("get item : %w", err)
	}
	if out.Item == nil {
		return Link{}, ErrNotFound
	}
	var item dynamoItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return Link{}, fmt.Errorf("unmarshal item: %w", err)
	}
	created, _ := time.Parse(time.RFC3339Nano, item.CreatedAt)
	return Link{
		Code:      item.Code,
		LongURL:   item.LongURL,
		CreatedAt: created,
	}, nil
}

func EnsureTable(ctx context.Context, client *dynamodb.Client, table string) error {
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(table),
	})
	if err == nil {
		return nil // table already exists
	}
	var notFound *types.ResourceNotFoundException
	if !errors.As(err, &notFound) {
		return fmt.Errorf("describe table : %w", err)
	}

	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(table),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("code"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("code"), KeyType: types.KeyTypeHash},
		},
	})
	if err != nil {
		return fmt.Errorf("create table: %w", err)
	}

	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(table),
	}, 30*time.Second); err != nil {
		return fmt.Errorf("wait for the table active: %w", err)
	}
	return nil
}
