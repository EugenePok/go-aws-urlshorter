package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"urlshortener/internal/events"
	"urlshortener/internal/stats"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeQueue struct {
	batch    []types.Message
	returned bool
	deleted  []types.DeleteMessageBatchRequestEntry
	delErr   error
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func msg(t *testing.T, code, handle string, ts time.Time) types.Message {
	t.Helper()
	body, err := json.Marshal(events.ClickEvent{Code: code, Timestamp: ts})
	if err != nil {
		t.Fatal(err)
	}
	return types.Message{Body: aws.String(string(body)), ReceiptHandle: aws.String(handle)}
}

func (f *fakeQueue) ReceiveMessage(_ context.Context, _ *sqs.ReceiveMessageInput,
	_ ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	if f.returned {
		return &sqs.ReceiveMessageOutput{}, nil
	}
	f.returned = true
	return &sqs.ReceiveMessageOutput{Messages: f.batch}, nil
}

func (f *fakeQueue) DeleteMessageBatch(_ context.Context, in *sqs.DeleteMessageBatchInput,
	_ ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error) {
	if f.delErr != nil {
		return nil, f.delErr
	}
	f.deleted = append(f.deleted, in.Entries...)
	return &sqs.DeleteMessageBatchOutput{}, nil
}

type fakeStats struct {
	applied []stats.Delta
	err     error
}

func (f *fakeStats) ApplyDeltas(_ context.Context, deltas []stats.Delta) error {
	if f.err != nil {
		return f.err
	}
	f.applied = append(f.applied, deltas...)
	return nil
}

func TestProcess_AggregatesByCode(t *testing.T) {
	now := time.Now().UTC()
	msgs := []types.Message{
		msg(t, "aaa", "h1", now),
		msg(t, "aaa", "h2", now.Add(time.Minute)),
		msg(t, "bbb", "h3", now),
	}
	deltas, toDelete := process(msgs, discardLogger())

	if len(toDelete) != 3 {
		t.Errorf("toDelete = %d, want 3", len(toDelete))
	}
	byCode := map[string]stats.Delta{}
	for _, d := range deltas {
		byCode[d.Code] = d
	}
	if byCode["aaa"].Count != 2 {
		t.Errorf("aaa count = %d, want 2", byCode["aaa"].Count)
	}
	if !byCode["aaa"].LastClicked.Equal(now.Add(time.Minute)) {
		t.Errorf("aaa LastClicked = %v, want latest in batch", byCode["aaa"].LastClicked)
	}
	if byCode["bbb"].Count != 1 {
		t.Errorf("bbb count = %d, want 1", byCode["bbb"].Count)
	}
}

func TestProcess_DropsMalformedButDeletesIt(t *testing.T) {
	msgs := []types.Message{
		{Body: aws.String("not json"), ReceiptHandle: aws.String("bad")},
		msg(t, "ok", "good", time.Now().UTC()),
	}
	deltas, toDelete := process(msgs, discardLogger())

	if len(deltas) != 1 || deltas[0].Code != "ok" {
		t.Errorf("deltas = %+v, want just the valid one", deltas)
	}
	if len(toDelete) != 2 {
		t.Errorf("toDelete = %d, want 2 (malformed is deleted too)", len(toDelete))
	}
}

func TestWorker_Poll_AppliesThenDeletes(t *testing.T) {
	q := &fakeQueue{batch: []types.Message{
		msg(t, "aaa", "h1", time.Now().UTC()),
		msg(t, "aaa", "h2", time.Now().UTC()),
	}}
	fs := &fakeStats{}
	w := New(q, "http://q", fs, discardLogger())

	if err := w.poll(context.Background()); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(fs.applied) != 1 || fs.applied[0].Count != 2 {
		t.Errorf("applied = %+v, want one delta with count 2", fs.applied)
	}
	if len(q.deleted) != 2 {
		t.Errorf("deleted = %d, want 2", len(q.deleted))
	}
}

func TestWorker_Poll_StatsErrorSkipsDelete(t *testing.T) {
	q := &fakeQueue{batch: []types.Message{msg(t, "aaa", "h1", time.Now().UTC())}}
	w := New(q, "http://q", &fakeStats{err: errors.New("db down")}, discardLogger())

	if err := w.poll(context.Background()); err == nil {
		t.Fatal("expected poll error when stats fail")
	}
	if len(q.deleted) != 0 {
		t.Errorf("deleted %d messages, want 0 (must not delete when DB write fails)", len(q.deleted))
	}
}
