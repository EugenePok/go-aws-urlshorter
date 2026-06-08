package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSClientConfig struct {
	Region   string
	Endpoint string
}

func NewSQSClient(ctx context.Context, cfg SQSClientConfig) (*sqs.Client, error) {
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
	return sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}), nil
}

type SQS struct {
	client   *sqs.Client
	queueURL string
}

func NewSQS(client *sqs.Client, queueURL string) *SQS {
	return &SQS{client: client, queueURL: queueURL}
}

func (s *SQS) PublishClick(ctx context.Context, event ClickEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal click event: %w", err)
	}
	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

func QueueURL(ctx context.Context, client *sqs.Client, name string) (string, error) {
	out, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("get queue url: %w", err)
	}
	return aws.ToString(out.QueueUrl), nil
}

func EnsureQueue(ctx context.Context, client *sqs.Client, name string) (string, error) {
	url, err := QueueURL(ctx, client, name)
	if err == nil {
		return url, nil
	}
	var notExists *types.QueueDoesNotExist
	if !errors.As(err, &notExists) {
		return "", err
	}
	out, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{QueueName: aws.String(name)})
	if err != nil {
		return "", fmt.Errorf("create queue: %w", err)
	}
	return aws.ToString(out.QueueUrl), nil
}
