//go: build integration

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func newTestDynamo(t *testing.T) *Dynamo {
	t.Helper()
	ctx := context.Background()

	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}
	client, err := NewDynamoClient(ctx, DynamoClientConfig{
		Region:   "us-east-1",
		Endpoint: endpoint,
	})
	if err != nil {
		t.Fatalf("NewDynamoClient: %v", err)
	}

	table := fmt.Sprintf("test_links_%d", time.Now().UnixNano())
	if err := EnsureTable(ctx, client, table); err != nil {
		t.Fatalf("EnsureTable: %v (is LocalStack up?)", err)
	}
	t.Cleanup(func() {
		_, _ = client.DeleteTable(ctx, &dynamodb.DeleteTableInput{
			TableName: aws.String(table),
		})
	})
	return NewDynamo(client, table)
}

func TestDynamo_SaveAndGet(t *testing.T) {
	d := newTestDynamo(t)
	ctx := context.Background()
	want := Link{Code: "abc1234", LongURL: "https://example.com?q=1", CreatedAt: time.Now().UTC().Truncate(time.Second)}

	if err := d.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := d.Get(ctx, want.Code)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LongURL != want.LongURL || got.Code != want.Code {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDynamo_get_NotFound(t *testing.T) {
	d := newTestDynamo(t)
	if _, err := d.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDynamo_Save_Duplicate(t *testing.T) {
	d := newTestDynamo(t)
	ctx := context.Background()
	link := Link{Code: "abc1234", LongURL: "https://example.com?q=1", CreatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := d.Save(ctx, link); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := d.Save(ctx, link); !errors.Is(err, ErrCodeExists) {
		t.Errorf("err = %v, want ErrCodeExists", err)
	}
}
