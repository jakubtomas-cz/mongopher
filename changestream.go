package mongopher

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ChangeEvent represents a single change stream event.
type ChangeEvent struct {
	OperationType string // "insert", "update", "replace", "delete", "drop", "invalidate"
	DocumentID    string // hex ObjectID of the affected document; empty for non-document events
	Document      []byte // full document JSON; nil for delete/drop/invalidate events
}

// ChangeStream is an iterator over change stream events.
type ChangeStream interface {
	// Next advances to the next event. Returns false when the stream is closed or the context is done.
	Next(ctx context.Context) bool
	// Event returns the current change event.
	Event() (ChangeEvent, error)
	// Close closes the stream and releases resources.
	Close(ctx context.Context) error
}

// WatchOption configures a Watch call.
type WatchOption func(*watchOptions)

type watchOptions struct {
	fullDocument   bool
	operationTypes []string
}

// WithFullDocument makes the stream include the full document on update events.
// Without this option, update events only indicate that a document changed, not its new state.
func WithFullDocument() WatchOption {
	return func(o *watchOptions) { o.fullDocument = true }
}

// WithOperationTypes filters the stream to only emit events of the given types.
// Valid types: "insert", "update", "replace", "delete", "drop", "invalidate".
func WithOperationTypes(types ...string) WatchOption {
	return func(o *watchOptions) { o.operationTypes = types }
}

type mongoChangeStream struct {
	inner *mongo.ChangeStream
}

func (cs *mongoChangeStream) Next(ctx context.Context) bool {
	return cs.inner.Next(ctx)
}

func (cs *mongoChangeStream) Event() (ChangeEvent, error) {
	var raw struct {
		OperationType string `bson:"operationType"`
		DocumentKey   struct {
			ID bson.ObjectID `bson:"_id"`
		} `bson:"documentKey"`
		FullDocument bson.D `bson:"fullDocument"`
	}
	if err := cs.inner.Decode(&raw); err != nil {
		return ChangeEvent{}, err
	}

	docID := ""
	if raw.DocumentKey.ID != (bson.ObjectID{}) {
		docID = raw.DocumentKey.ID.Hex()
	}
	ev := ChangeEvent{
		OperationType: raw.OperationType,
		DocumentID:    docID,
	}

	if raw.FullDocument != nil {
		doc, err := bsonToJSON(raw.FullDocument)
		if err != nil {
			return ChangeEvent{}, err
		}
		ev.Document = doc
	}

	return ev, nil
}

func (cs *mongoChangeStream) Close(ctx context.Context) error {
	return cs.inner.Close(ctx)
}

