package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"
	"urlshortener/internal/events"
	"urlshortener/internal/stats"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Queue interface {
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput,
		...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageBatch(context.Context, *sqs.DeleteMessageBatchInput,
		...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error)
}

type Worker struct {
	queue    Queue
	queueURL string
	stats    stats.Store
	log      *slog.Logger

	waitTime int32 // SQS long-poll seconds
	maxMsgs  int32 // messages per receive (<=10)
}

func New(queue Queue, queueURL string, store stats.Store, log *slog.Logger) *Worker {
	return &Worker{
		queue:    queue,
		queueURL: queueURL,
		stats:    store,
		log:      log,
		waitTime: 20,
		maxMsgs:  10,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("worker started", "queue", w.queueURL)
	for {
		if ctx.Err() != nil {
			w.log.Info("worker stopping")
			return nil
		}
		if err := w.poll(ctx); err != nil {
			if ctx.Err() != nil {
				w.log.Info("worker stopping")
				return nil
			}
			w.log.Error("poll failed; backing off", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
}

func (w *Worker) poll(ctx context.Context) error {
	out, err := w.queue.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &w.queueURL,
		WaitTimeSeconds:     w.waitTime,
		MaxNumberOfMessages: w.maxMsgs,
	})
	if err != nil {
		return err
	}
	if len(out.Messages) == 0 {
		return nil
	}

	deltas, toDelete := process(out.Messages, w.log)
	if len(deltas) > 0 {
		if err := w.stats.ApplyDeltas(ctx, deltas); err != nil {
			return err
		}
	}
	return w.deleteBatch(ctx, toDelete)
}

func process(msgs []types.Message, log *slog.Logger) ([]stats.Delta, []types.DeleteMessageBatchRequestEntry) {
	agg := make(map[string]*stats.Delta, len(msgs))
	toDelete := make([]types.DeleteMessageBatchRequestEntry, 0, len(msgs))

	for i, m := range msgs {
		entry := types.DeleteMessageBatchRequestEntry{
			Id:            aws.String(strconv.Itoa(i)),
			ReceiptHandle: m.ReceiptHandle,
		}

		var ev events.ClickEvent
		if err := json.Unmarshal([]byte(aws.ToString(m.Body)), &ev); err != nil {
			// Poison message: delete it so it can't block the queue forever.
			// (A DLQ would be the production choice)
			log.Warn("dropping malformed message", "err", err)
			toDelete = append(toDelete, entry)
			continue
		}

		d := agg[ev.Code]
		if d == nil {
			d = &stats.Delta{Code: ev.Code}
			agg[ev.Code] = d
		}
		d.Count++
		ts := ev.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if ts.After(d.LastClicked) {
			d.LastClicked = ts
		}
		toDelete = append(toDelete, entry)
	}

	deltas := make([]stats.Delta, 0, len(agg))
	for _, d := range agg {
		deltas = append(deltas, *d)
	}
	return deltas, toDelete
}

func (w *Worker) deleteBatch(ctx context.Context, entries []types.DeleteMessageBatchRequestEntry) error {
	if len(entries) == 0 {
		return nil
	}
	out, err := w.queue.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(w.queueURL),
		Entries:  entries,
	})
	if err != nil {
		return nil
	}
	if len(out.Failed) > 0 {
		// These reappear after the visibility timeout and get reprocessed.
		w.log.Warn("some message deletes failed", "count", len(out.Failed))
	}
	return nil
}
