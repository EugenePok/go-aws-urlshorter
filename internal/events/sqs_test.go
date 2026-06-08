//go:build integration

package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func TestSQS_PublishAndReceive(t *testing.T) {
	ctx := context.Background()
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}

	client, err := NewSQSClient(ctx, SQSClientConfig{
		Region:   "us-east-1",
		Endpoint: endpoint,
	})
	if err != nil {
		t.Fatalf("NewSQSClient : %v", err)
	}

	queueURL, err := EnsureQueue(ctx, client, fmt.Sprintf("test_clicks_%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("EnsureQueue: %v (is LocalStack up with sqs?)", err)
	}
	t.Cleanup(func() {
		_, _ = client.DeleteQueue(ctx, &sqs.DeleteQueueInput{QueueUrl: aws.String(queueURL)})
	})

	pub := NewSQS(client, queueURL)
	want := ClickEvent{
		Code:      "abc1234",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		UserAgent: "go-test"}
	if err := pub.PublishClick(ctx, want); err != nil {
		t.Fatalf("PublishClick: %v", err)
	}

	out, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     5,
	})
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}
	if len(out.Messages) != 1 {
		t.Fatalf("received %d messages, want 1", len(out.Messages))
	}

	var got ClickEvent
	if err := json.Unmarshal([]byte(aws.ToString(out.Messages[0].Body)), &got); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}

	if got.Code != want.Code || got.UserAgent != want.UserAgent {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
