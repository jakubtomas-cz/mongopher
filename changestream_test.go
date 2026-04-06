package mongopher_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jakubtomas-cz/mongopher"
)

// watchCtx returns a context that cancels after 5 seconds, enough to receive expected events.
func watchCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func TestWatchInsert(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	cs, err := c.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		_, _ = c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.OperationType != "insert" {
		t.Fatalf("expected insert, got %q", ev.OperationType)
	}
	if ev.DocumentID == "" {
		t.Fatal("expected non-empty DocumentID")
	}
}

func TestWatchInsertDocument(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	cs, err := c.Watch(ctx, mongopher.WithFullDocument())
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		_, _ = c.InsertOne(context.Background(), []byte(`{"name":"Alice","age":30}`))
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.Document == nil {
		t.Fatal("expected full document")
	}
	var doc map[string]any
	if err := json.Unmarshal(ev.Document, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Alice" {
		t.Fatalf("expected name Alice, got %v", doc["name"])
	}
}

func TestWatchUpdate(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	res, err := c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	cs, err := c.Watch(ctx, mongopher.WithFullDocument())
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		f, _ := mongopher.FilterByID(res.InsertedID)
		_, _ = c.UpdateOne(context.Background(), f, mongopher.Set([]byte(`{"name":"Bob"}`)))
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.OperationType != "update" {
		t.Fatalf("expected update, got %q", ev.OperationType)
	}
	if ev.DocumentID != res.InsertedID {
		t.Fatalf("expected DocumentID %q, got %q", res.InsertedID, ev.DocumentID)
	}
	if ev.Document == nil {
		t.Fatal("expected full document with WithFullDocument")
	}
	var doc map[string]any
	if err := json.Unmarshal(ev.Document, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Bob" {
		t.Fatalf("expected name Bob, got %v", doc["name"])
	}
}

func TestWatchUpdateWithoutFullDocument(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	res, err := c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	cs, err := c.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		f, _ := mongopher.FilterByID(res.InsertedID)
		_, _ = c.UpdateOne(context.Background(), f, mongopher.Set([]byte(`{"name":"Bob"}`)))
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.OperationType != "update" {
		t.Fatalf("expected update, got %q", ev.OperationType)
	}
	if ev.Document != nil {
		t.Fatal("expected nil document without WithFullDocument")
	}
}

func TestWatchDelete(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	res, err := c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	cs, err := c.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		f, _ := mongopher.FilterByID(res.InsertedID)
		_, _ = c.DeleteOne(context.Background(), f)
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.OperationType != "delete" {
		t.Fatalf("expected delete, got %q", ev.OperationType)
	}
	if ev.DocumentID != res.InsertedID {
		t.Fatalf("expected DocumentID %q, got %q", res.InsertedID, ev.DocumentID)
	}
	if ev.Document != nil {
		t.Fatal("expected nil document for delete event")
	}
}

func TestWatchOperationTypes(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)
	cs, err := c.Watch(ctx, mongopher.WithOperationTypes("delete"))
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		res, _ := c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
		f, _ := mongopher.FilterByID(res.InsertedID)
		_, _ = c.DeleteOne(context.Background(), f)
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected an event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	// Only the delete should come through; insert was filtered out
	if ev.OperationType != "delete" {
		t.Fatalf("expected delete, got %q", ev.OperationType)
	}
}

func TestWatchDropEmptyDocumentID(t *testing.T) {
	ctx, cancel := watchCtx(t)
	defer cancel()

	c := col(t)

	// Insert a document so the collection exists before watching
	_, err := c.InsertOne(context.Background(), []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	cs, err := c.Watch(ctx, mongopher.WithOperationTypes("drop"))
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close(context.Background())

	go func() {
		_ = c.Drop(context.Background())
	}()

	if !cs.Next(ctx) {
		t.Fatal("expected a drop event")
	}
	ev, err := cs.Event()
	if err != nil {
		t.Fatal(err)
	}
	if ev.OperationType != "drop" {
		t.Fatalf("expected drop, got %q", ev.OperationType)
	}
	if ev.DocumentID != "" {
		t.Fatalf("expected empty DocumentID for drop event, got %q", ev.DocumentID)
	}
}
