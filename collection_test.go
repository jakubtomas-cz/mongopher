package mongopher_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jakubtomas-cz/mongopher"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	testURI = "mongodb://localhost:27017/?replicaSet=rs0"
	testDB  = "mongopher_test"
)

var testClient *mongopher.Client

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error
	testClient, err = mongopher.Connect(ctx, testURI, testDB)
	if err != nil {
		panic("failed to connect to MongoDB: " + err.Error())
	}

	code := m.Run()

	// Drop test database after suite
	_ = testClient.Disconnect(ctx)
	os.Exit(code)
}

func col(t *testing.T) mongopher.Collection {
	t.Helper()
	c := testClient.Collection(t.Name())
	// Clean up the collection before each test
	t.Cleanup(func() {
		_ = testClient.Collection(t.Name()).Drop(context.Background())
	})
	return c
}

func TestInsertOne(t *testing.T) {
	c := col(t)
	res, err := c.InsertOne(context.Background(), []byte(`{"name":"Alice","age":30}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.InsertedID == "" {
		t.Fatal("expected non-empty InsertedID")
	}
}

func TestInsertMany(t *testing.T) {
	c := col(t)
	res, err := c.InsertMany(context.Background(), []byte(`[{"name":"Alice"},{"name":"Bob"},{"name":"Carol"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.InsertedIDs) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(res.InsertedIDs))
	}
	for i, id := range res.InsertedIDs {
		if id == "" {
			t.Fatalf("InsertedIDs[%d] is empty", i)
		}
	}
}

func TestFindOne(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, err := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(doc, &result); err != nil {
		t.Fatal(err)
	}
	if result["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", result["name"])
	}
}

func TestFindOne_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	_, err := c.FindOne(ctx, filter)
	if !errors.Is(err, mongopher.ErrNoDocuments) {
		t.Fatalf("expected ErrNoDocuments, got %v", err)
	}
}

func TestFind(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"role":"admin","name":"Alice"},{"role":"admin","name":"Bob"},{"role":"user","name":"Carol"}]`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"admin"}`))
	results, err := c.Find(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 2 {
		t.Fatalf("expected 2 results, got %d", len(items))
	}
}

func TestUpdateOne(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	res, err := c.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
	if err != nil {
		t.Fatal(err)
	}
	if res.ModifiedCount != 1 {
		t.Fatalf("expected ModifiedCount=1, got %d", res.ModifiedCount)
	}

	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(doc, &result); err != nil {
		t.Fatal(err)
	}
	// JSON numbers unmarshal as float64
	if result["age"] != float64(31) {
		t.Fatalf("expected age=31, got %v", result["age"])
	}
}

func TestDeleteOne(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	res, err := c.DeleteOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedCount != 1 {
		t.Fatalf("expected DeletedCount=1, got %d", res.DeletedCount)
	}

	_, err = c.FindOne(ctx, filter)
	if !errors.Is(err, mongopher.ErrNoDocuments) {
		t.Fatalf("expected ErrNoDocuments after delete, got %v", err)
	}
}

func TestUpdateMany(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"role":"admin","score":10},{"role":"admin","score":10},{"role":"user","score":10}]`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"admin"}`))
	res, err := c.UpdateMany(ctx, filter, mongopher.Set([]byte(`{"score":99}`)))
	if err != nil {
		t.Fatal(err)
	}
	if res.ModifiedCount != 2 {
		t.Fatalf("expected ModifiedCount=2, got %d", res.ModifiedCount)
	}

	results, err := c.Find(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range parseArray(t, results) {
		var m map[string]any
		json.Unmarshal(doc, &m)
		if m["score"] != float64(99) {
			t.Fatalf("expected score=99, got %v", m["score"])
		}
	}
}

func TestDeleteMany(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"role":"admin"},{"role":"admin"},{"role":"user"}]`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"admin"}`))
	res, err := c.DeleteMany(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedCount != 2 {
		t.Fatalf("expected DeletedCount=2, got %d", res.DeletedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining document, got %d", count)
	}
}

func TestCountDocuments(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"type":"a"},{"type":"a"},{"type":"b"}]`)); err != nil {
		t.Fatal(err)
	}

	total, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"type":"a"}`))
	count, err := c.CountDocuments(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}
}

func TestFind_Empty(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	results, err := c.Find(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 0 {
		t.Fatalf("expected empty array, got %d results", len(items))
	}
}

func TestFind_EmptyFilter(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"n":1},{"n":2},{"n":3}]`)); err != nil {
		t.Fatal(err)
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 3 {
		t.Fatalf("expected 3 results, got %d", len(items))
	}
}

func TestFind_WithLimit(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	for range 5 {
		c.InsertOne(ctx, []byte(`{"x":1}`))
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithLimit(3))
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 3 {
		t.Fatalf("expected 3 results with limit, got %d", len(items))
	}
}

func TestFind_WithSkip(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	for range 5 {
		c.InsertOne(ctx, []byte(`{"x":1}`))
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithSkip(3))
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 2 {
		t.Fatalf("expected 2 results after skip, got %d", len(items))
	}
}

func TestFind_WithSort(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"score":3},{"score":1},{"score":2}]`)); err != nil {
		t.Fatal(err)
	}

	// Ascending
	results, err := c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithSort("score", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	items := parseArray(t, results)
	scores := make([]float64, len(items))
	for i, raw := range items {
		var m map[string]any
		json.Unmarshal(raw, &m)
		scores[i] = m["score"].(float64)
	}
	if scores[0] != 1 || scores[1] != 2 || scores[2] != 3 {
		t.Fatalf("expected ascending order [1,2,3], got %v", scores)
	}

	// Descending
	results, err = c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithSort("score", mongopher.DESC))
	if err != nil {
		t.Fatal(err)
	}
	for i, raw := range parseArray(t, results) {
		var m map[string]any
		json.Unmarshal(raw, &m)
		scores[i] = m["score"].(float64)
	}
	if scores[0] != 3 || scores[1] != 2 || scores[2] != 1 {
		t.Fatalf("expected descending order [3,2,1], got %v", scores)
	}
}

func TestFind_WithSort_MultiField(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"role":"admin","name":"Charlie"},{"role":"admin","name":"Alice"},{"role":"user","name":"Bob"},{"role":"user","name":"Alice"}]`)); err != nil {
		t.Fatal(err)
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter(),
		mongopher.WithSort("role", mongopher.ASC), // role ASC
		mongopher.WithSort("name", mongopher.ASC), // then name ASC
	)
	if err != nil {
		t.Fatal(err)
	}
	items := parseArray(t, results)
	if len(items) != 4 {
		t.Fatalf("expected 4 results, got %d", len(items))
	}

	type doc struct {
		Role string `json:"role"`
		Name string `json:"name"`
	}
	want := []doc{
		{"admin", "Alice"},
		{"admin", "Charlie"},
		{"user", "Alice"},
		{"user", "Bob"},
	}
	for i, raw := range items {
		var got doc
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got != want[i] {
			t.Fatalf("result[%d]: expected %+v, got %+v", i, want[i], got)
		}
	}
}

func TestInsertOne_InvalidJSON(t *testing.T) {
	c := col(t)
	_, err := c.InsertOne(context.Background(), []byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestUpdateOne_InvalidJSON(t *testing.T) {
	c := col(t)
	_, err := c.UpdateOne(context.Background(), mongopher.EmptyFilter(), []byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestUpdateOne_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	res, err := c.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}
}

func TestInsertOne_NestedDocument(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"user":{"name":"Alice","address":{"city":"Prague"}}}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"user.address.city":"Prague"}`))
	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(doc, &result)
	user := result["user"].(map[string]any)
	addr := user["address"].(map[string]any)
	if addr["city"] != "Prague" {
		t.Fatalf("expected city=Prague, got %v", addr["city"])
	}
}

func TestInsertOne_CustomID(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"_id":"my-custom-id","name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"_id":"my-custom-id"}`))
	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["_id"] != "my-custom-id" {
		t.Fatalf("expected _id=my-custom-id, got %v", result["_id"])
	}
}

func TestWithTransaction_Commit(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	err := testClient.WithTransaction(ctx, func(ctx context.Context) error {
		_, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document after commit, got %d", count)
	}
}

func TestWithTransaction_RegularErrorIsNotTransactionsNotSupported(t *testing.T) {
	ctx := context.Background()
	sentinel := errors.New("intentional error")

	err := testClient.WithTransaction(ctx, func(ctx context.Context) error {
		return sentinel
	})

	if errors.Is(err, mongopher.ErrReplicaSetRequired) {
		t.Fatal("regular transaction error must not be ErrReplicaSetRequired")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error to be preserved, got %v", err)
	}
}

func TestWithTransaction_Rollback(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	err := testClient.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`)); err != nil {
			return err
		}
		return errors.New("intentional rollback")
	})
	if err == nil {
		t.Fatal("expected error from transaction, got nil")
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 documents after rollback, got %d", count)
	}
}

func TestWithTransaction_MultiCollection(t *testing.T) {
	ctx := context.Background()
	orders := col(t)
	// col() names the collection after the test — use a second collection manually
	inventory := testClient.Collection(t.Name() + "_inventory")
	t.Cleanup(func() { _ = inventory.Drop(context.Background()) })

	_, err := inventory.InsertOne(ctx, []byte(`{"sku":"ABC","stock":10}`))
	if err != nil {
		t.Fatal(err)
	}

	err = testClient.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := orders.InsertOne(ctx, []byte(`{"sku":"ABC","qty":2}`)); err != nil {
			return err
		}
		filter, _ := mongopher.FilterFromJSON([]byte(`{"sku":"ABC"}`))
		_, err := inventory.UpdateOne(ctx, filter, mongopher.Inc([]byte(`{"stock":-2}`)))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"sku":"ABC"}`))
	doc, err := inventory.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["stock"] != float64(8) {
		t.Fatalf("expected stock=8 after transaction, got %v", result["stock"])
	}
}

func TestWithTransaction_Collection_Commit(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	err := c.WithTransaction(ctx, func(ctx context.Context) error {
		_, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document after commit, got %d", count)
	}
}

func TestWithTransaction_Collection_Rollback(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	err := c.WithTransaction(ctx, func(ctx context.Context) error {
		if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`)); err != nil {
			return err
		}
		return errors.New("intentional rollback")
	})
	if err == nil {
		t.Fatal("expected error from transaction, got nil")
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 documents after rollback, got %d", count)
	}
}

func TestConnect_WithNoAdditionalOpts(t *testing.T) {
	ctx := context.Background()
	client, err := mongopher.Connect(ctx, testURI, testDB)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
}

func TestConnect_WithAdditionalOpt(t *testing.T) {
	ctx := context.Background()
	opt := options.Client().SetAppName("mongopher-test")
	client, err := mongopher.Connect(ctx, testURI, testDB, opt)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
}

func TestConnect_WithMultipleAdditionalOpts(t *testing.T) {
	ctx := context.Background()
	opt1 := options.Client().SetAppName("mongopher-test")
	opt2 := options.Client().SetServerSelectionTimeout(5 * time.Second)
	client, err := mongopher.Connect(ctx, testURI, testDB, opt1, opt2)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
}

func TestConnect_BadURI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := mongopher.Connect(ctx, "mongodb://localhost:1", "test")
	if err == nil {
		t.Fatal("expected error connecting to unreachable host, got nil")
	}
}

func TestConnect_InvalidURI(t *testing.T) {
	ctx := context.Background()
	_, err := mongopher.Connect(ctx, "not-a-uri", "test")
	if err == nil {
		t.Fatal("expected error on invalid URI, got nil")
	}
}

func TestDisconnect(t *testing.T) {
	ctx := context.Background()
	client, err := mongopher.Connect(ctx, testURI, testDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Disconnect(ctx); err != nil {
		t.Fatalf("expected clean disconnect, got: %v", err)
	}
}

func TestDrop(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"x":1},{"x":2}]`)); err != nil {
		t.Fatal(err)
	}

	if err := c.Drop(ctx); err != nil {
		t.Fatal(err)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 documents after drop, got %d", count)
	}
}

func TestInsertMany_InvalidDoc(t *testing.T) {
	c := col(t)
	_, err := c.InsertMany(context.Background(), []byte(`[{"name":"Alice"},not json,{"name":"Carol"}]`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestFilterFromJSON_Invalid(t *testing.T) {
	_, err := mongopher.FilterFromJSON([]byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestFilterFromJSON_ObjectIDCoercion(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	// Insert without custom _id so MongoDB assigns an ObjectID.
	res, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	// Build a filter from the hex string returned by InsertOne, as a user
	// would after receiving a document over HTTP.
	filter, err := mongopher.FilterFromJSON([]byte(`{"_id":"` + res.InsertedID + `"}`))
	if err != nil {
		t.Fatal(err)
	}

	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", result["name"])
	}
}

func TestFilterFromJSON_CustomStringID(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"_id":"user-42","name":"Bob"}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, err := mongopher.FilterFromJSON([]byte(`{"_id":"user-42"}`))
	if err != nil {
		t.Fatal(err)
	}

	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["name"] != "Bob" {
		t.Fatalf("expected name=Bob, got %v", result["name"])
	}
}

// parseArray is a test helper that unmarshals a JSON array into individual raw documents.
func parseArray(t *testing.T, docs []byte) []json.RawMessage {
	t.Helper()
	items, err := mongopher.UnmarshalAs[[]json.RawMessage](docs)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func names(docs []byte) []string {
	items, _ := mongopher.UnmarshalAs[[]map[string]any](docs)
	var out []string
	for _, m := range items {
		if n, ok := m["name"].(string); ok {
			out = append(out, n)
		}
	}
	return out
}

func seedDocs(t *testing.T, c mongopher.Collection, docs ...string) {
	t.Helper()
	ctx := context.Background()
	for _, d := range docs {
		if _, err := c.InsertOne(ctx, []byte(d)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEq_ObjectIDCoercion(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	res, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	doc, err := c.FindOne(ctx, mongopher.Eq("_id", res.InsertedID))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", result["name"])
	}
}

func TestEq(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","status":"active"}`,
		`{"name":"Bob","status":"inactive"}`,
	)

	docs, err := c.Find(ctx, mongopher.Eq("status", "active"))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Alice" {
		t.Fatalf("expected [Alice], got %v", n)
	}
}

func TestNe(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","status":"active"}`,
		`{"name":"Bob","status":"inactive"}`,
	)

	docs, err := c.Find(ctx, mongopher.Ne("status", "active"))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Bob" {
		t.Fatalf("expected [Bob], got %v", n)
	}
}

func TestGt(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","age":20}`,
		`{"name":"Bob","age":30}`,
		`{"name":"Carol","age":40}`,
	)

	docs, err := c.Find(ctx, mongopher.Gt("age", 25), mongopher.WithSort("age", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 2 || n[0] != "Bob" || n[1] != "Carol" {
		t.Fatalf("expected [Bob Carol], got %v", n)
	}
}

func TestGte(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","age":20}`,
		`{"name":"Bob","age":30}`,
		`{"name":"Carol","age":40}`,
	)

	docs, err := c.Find(ctx, mongopher.Gte("age", 30), mongopher.WithSort("age", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 2 || n[0] != "Bob" || n[1] != "Carol" {
		t.Fatalf("expected [Bob Carol], got %v", n)
	}
}

func TestLt(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","age":20}`,
		`{"name":"Bob","age":30}`,
		`{"name":"Carol","age":40}`,
	)

	docs, err := c.Find(ctx, mongopher.Lt("age", 35), mongopher.WithSort("age", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 2 || n[0] != "Alice" || n[1] != "Bob" {
		t.Fatalf("expected [Alice Bob], got %v", n)
	}
}

func TestLte(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","age":20}`,
		`{"name":"Bob","age":30}`,
		`{"name":"Carol","age":40}`,
	)

	docs, err := c.Find(ctx, mongopher.Lte("age", 30), mongopher.WithSort("age", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 2 || n[0] != "Alice" || n[1] != "Bob" {
		t.Fatalf("expected [Alice Bob], got %v", n)
	}
}

func TestIn(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","role":"admin"}`,
		`{"name":"Bob","role":"user"}`,
		`{"name":"Carol","role":"owner"}`,
	)

	docs, err := c.Find(ctx, mongopher.In("role", "admin", "owner"), mongopher.WithSort("name", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 2 || n[0] != "Alice" || n[1] != "Carol" {
		t.Fatalf("expected [Alice Carol], got %v", n)
	}
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","deletedAt":"2024-01-01"}`,
		`{"name":"Bob"}`,
	)

	docs, err := c.Find(ctx, mongopher.Exists("deletedAt", false))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Bob" {
		t.Fatalf("expected [Bob], got %v", n)
	}

	docs, err = c.Find(ctx, mongopher.Exists("deletedAt", true))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Alice" {
		t.Fatalf("expected [Alice], got %v", n)
	}
}

func TestAnd(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","status":"active","age":20}`,
		`{"name":"Bob","status":"active","age":40}`,
		`{"name":"Carol","status":"inactive","age":40}`,
	)

	docs, err := c.Find(ctx, mongopher.And(
		mongopher.Eq("status", "active"),
		mongopher.Gt("age", 30),
	))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Bob" {
		t.Fatalf("expected [Bob], got %v", n)
	}
}

func TestAnd_SameField(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","age":10}`,
		`{"name":"Bob","age":30}`,
		`{"name":"Carol","age":50}`,
	)

	docs, err := c.Find(ctx, mongopher.And(
		mongopher.Gt("age", 20),
		mongopher.Lt("age", 40),
	))
	if err != nil {
		t.Fatal(err)
	}
	if n := names(docs); len(n) != 1 || n[0] != "Bob" {
		t.Fatalf("expected [Bob], got %v", n)
	}
}

func TestOr(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c,
		`{"name":"Alice","role":"admin"}`,
		`{"name":"Bob","role":"viewer"}`,
		`{"name":"Carol","role":"owner"}`,
	)

	docs, err := c.Find(ctx, mongopher.Or(
		mongopher.Eq("role", "admin"),
		mongopher.Eq("role", "owner"),
	))
	if err != nil {
		t.Fatal(err)
	}
	n := names(docs)
	if len(n) != 2 {
		t.Fatalf("expected 2 docs, got %v", n)
	}
	got := map[string]bool{n[0]: true, n[1]: true}
	if !got["Alice"] || !got["Carol"] {
		t.Fatalf("expected [Alice Carol], got %v", n)
	}
}

func TestWithFields_Find(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c, `{"name":"Alice","age":30,"role":"admin"}`)

	docs, err := c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithFields("name", "role"))
	if err != nil {
		t.Fatal(err)
	}
	items, _ := mongopher.UnmarshalAs[[]map[string]any](docs)
	if len(items) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(items))
	}
	item := items[0]
	if _, ok := item["name"]; !ok {
		t.Error("expected 'name' field")
	}
	if _, ok := item["role"]; !ok {
		t.Error("expected 'role' field")
	}
	if _, ok := item["age"]; ok {
		t.Error("expected 'age' to be excluded")
	}
}

func TestWithFields_FindOne(t *testing.T) {
	ctx := context.Background()
	c := col(t)
	seedDocs(t, c, `{"name":"Alice","age":30,"role":"admin"}`)

	doc, err := c.FindOne(ctx, mongopher.EmptyFilter(), mongopher.WithFields("name"))
	if err != nil {
		t.Fatal(err)
	}
	item, _ := mongopher.UnmarshalAs[map[string]any](doc)
	if _, ok := item["name"]; !ok {
		t.Error("expected 'name' field")
	}
	if _, ok := item["age"]; ok {
		t.Error("expected 'age' to be excluded")
	}
	if _, ok := item["role"]; ok {
		t.Error("expected 'role' to be excluded")
	}
}

func TestFilterByID(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	// custom string _id
	res, err := c.InsertOne(ctx, []byte(`{"_id":"user-42","name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}
	filter, err := mongopher.FilterByID(res.InsertedID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", result["name"])
	}

	// MongoDB-generated ObjectID
	res, err = c.InsertOne(ctx, []byte(`{"name":"Bob"}`))
	if err != nil {
		t.Fatal(err)
	}
	filter, err = mongopher.FilterByID(res.InsertedID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(doc, &result)
	if result["name"] != "Bob" {
		t.Fatalf("expected name=Bob, got %v", result["name"])
	}
}

func TestIDFlattening(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.InsertOne(ctx, []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	if err := json.Unmarshal(doc, &result); err != nil {
		t.Fatal(err)
	}

	id, ok := result["_id"]
	if !ok {
		t.Fatal("_id field missing from result")
	}
	if _, isString := id.(string); !isString {
		t.Fatalf("expected _id to be a plain string, got %T: %v", id, id)
	}
}

func TestAggregate_GroupAndCount(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"city":"Prague","status":"active"},{"city":"Prague","status":"active"},{"city":"Brno","status":"active"},{"city":"Brno","status":"inactive"},{"city":"Brno","status":"inactive"}]`)); err != nil {
		t.Fatal(err)
	}

	pipeline := []byte(`[
		{"$group": {"_id": "$city", "count": {"$sum": 1}}},
		{"$sort": {"count": -1}}
	]`)
	results, err := c.Aggregate(ctx, pipeline)
	if err != nil {
		t.Fatal(err)
	}
	type group struct {
		ID    string  `json:"_id"`
		Count float64 `json:"count"`
	}
	groups, err := mongopher.UnmarshalAs[[]group](results)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].ID != "Brno" || groups[0].Count != 3 {
		t.Fatalf("expected Brno=3, got %s=%v", groups[0].ID, groups[0].Count)
	}
	if groups[1].ID != "Prague" || groups[1].Count != 2 {
		t.Fatalf("expected Prague=2, got %s=%v", groups[1].ID, groups[1].Count)
	}
}

func TestAggregate_MatchAndProject(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"name":"Alice","score":80},{"name":"Bob","score":40},{"name":"Carol","score":95}]`)); err != nil {
		t.Fatal(err)
	}

	pipeline := []byte(`[
		{"$match": {"score": {"$gte": 70}}},
		{"$project": {"_id": 0, "name": 1}}
	]`)
	results, err := c.Aggregate(ctx, pipeline)
	if err != nil {
		t.Fatal(err)
	}
	items := parseArray(t, results)
	if len(items) != 2 {
		t.Fatalf("expected 2 results after $match, got %d", len(items))
	}
	for _, r := range items {
		var doc map[string]any
		json.Unmarshal(r, &doc)
		if _, hasID := doc["_id"]; hasID {
			t.Fatal("expected _id to be projected out")
		}
		if _, hasName := doc["name"]; !hasName {
			t.Fatal("expected name field in projected result")
		}
	}
}

func TestAggregate_EmptyResult(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}

	pipeline := []byte(`[{"$match": {"x": 999}}]`)
	results, err := c.Aggregate(ctx, pipeline)
	if err != nil {
		t.Fatal(err)
	}
	if items := parseArray(t, results); len(items) != 0 {
		t.Fatalf("expected empty aggregate result, got %d items", len(items))
	}
}

func TestAggregate_InvalidPipeline(t *testing.T) {
	c := col(t)
	_, err := c.Aggregate(context.Background(), []byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestCreateIndex(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	name, err := c.CreateIndex(ctx, []mongopher.IndexKey{{Field: "email", Direction: mongopher.ASC}})
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("expected non-empty index name")
	}
}

func TestCreateIndex_Unique(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.CreateIndex(ctx, []mongopher.IndexKey{{Field: "email", Direction: mongopher.ASC}}, mongopher.WithUnique())
	if err != nil {
		t.Fatal(err)
	}

	// First insert succeeds
	if _, err := c.InsertOne(ctx, []byte(`{"email":"alice@example.com"}`)); err != nil {
		t.Fatal(err)
	}
	// Duplicate insert must fail
	_, err = c.InsertOne(ctx, []byte(`{"email":"alice@example.com"}`))
	if err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
}

func TestCreateIndex_Desc(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	name, err := c.CreateIndex(ctx, []mongopher.IndexKey{{Field: "createdAt", Direction: mongopher.DESC}})
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("expected non-empty index name")
	}
}

func TestCreateIndex_Compound(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	name, err := c.CreateIndex(ctx, []mongopher.IndexKey{
		{Field: "role", Direction: mongopher.ASC},
		{Field: "createdAt", Direction: mongopher.DESC},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("expected non-empty index name")
	}
}

func TestCreateIndex_CompoundUnique(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	_, err := c.CreateIndex(ctx, []mongopher.IndexKey{
		{Field: "org", Direction: mongopher.ASC},
		{Field: "email", Direction: mongopher.ASC},
	}, mongopher.WithUnique())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.InsertOne(ctx, []byte(`{"org":"acme","email":"alice@example.com"}`)); err != nil {
		t.Fatal(err)
	}
	// Same org+email must fail
	_, err = c.InsertOne(ctx, []byte(`{"org":"acme","email":"alice@example.com"}`))
	if err == nil {
		t.Fatal("expected duplicate key error, got nil")
	}
	// Different org same email must succeed
	if _, err := c.InsertOne(ctx, []byte(`{"org":"globex","email":"alice@example.com"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestListIndexes(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	// MongoDB always creates a default _id index
	if _, err := c.InsertOne(ctx, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateIndex(ctx, []mongopher.IndexKey{{Field: "email", Direction: mongopher.ASC}}, mongopher.WithUnique()); err != nil {
		t.Fatal(err)
	}

	indexes, err := c.ListIndexes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idxItems := parseArray(t, indexes)
	// Expect at least the _id index and our new one
	if len(idxItems) < 2 {
		t.Fatalf("expected at least 2 indexes, got %d", len(idxItems))
	}
	for _, idx := range idxItems {
		var doc map[string]any
		if err := json.Unmarshal(idx, &doc); err != nil {
			t.Fatalf("index is not valid JSON: %v", err)
		}
		if _, hasName := doc["name"]; !hasName {
			t.Fatal("expected index document to have a name field")
		}
	}
}

func TestDropIndex(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}

	name, err := c.CreateIndex(ctx, []mongopher.IndexKey{{Field: "email", Direction: mongopher.ASC}})
	if err != nil {
		t.Fatal(err)
	}

	before, err := c.ListIndexes(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := c.DropIndex(ctx, name); err != nil {
		t.Fatal(err)
	}

	after, err := c.ListIndexes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	beforeItems, afterItems := parseArray(t, before), parseArray(t, after)
	if len(afterItems) != len(beforeItems)-1 {
		t.Fatalf("expected %d indexes after drop, got %d", len(beforeItems)-1, len(afterItems))
	}
}

func TestUpdateOne_WithUpsert_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Ghost"}`))
	res, err := c.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"name":"Ghost","age":0}`)), mongopher.WithUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upserted document, got %d", count)
	}
}

func TestUpdateOne_WithUpsert_Match(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	res, err := c.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)), mongopher.WithUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 1 || res.ModifiedCount != 1 {
		t.Fatalf("expected MatchedCount=1 ModifiedCount=1, got %d/%d", res.MatchedCount, res.ModifiedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 document (no duplicate inserted), got %d", count)
	}
}

func TestUpdateMany_WithUpsert_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"ghost"}`))
	res, err := c.UpdateMany(ctx, filter, mongopher.Set([]byte(`{"role":"ghost"}`)), mongopher.WithUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upserted document, got %d", count)
	}
}

func TestReplaceOne(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30,"role":"admin"}`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	res, err := c.ReplaceOne(ctx, filter, []byte(`{"name":"Alice","age":31}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 1 || res.ModifiedCount != 1 {
		t.Fatalf("expected MatchedCount=1 ModifiedCount=1, got %d/%d", res.MatchedCount, res.ModifiedCount)
	}

	doc, err := c.FindOne(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["age"] != float64(31) {
		t.Fatalf("expected age=31, got %v", result["age"])
	}
	if _, hasRole := result["role"]; hasRole {
		t.Fatal("expected role field to be gone after replace")
	}
}

func TestReplaceOne_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	res, err := c.ReplaceOne(ctx, filter, []byte(`{"name":"nobody"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}
}

func TestReplaceOne_WithUpsert(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Ghost"}`))
	res, err := c.ReplaceOne(ctx, filter, []byte(`{"name":"Ghost","score":0}`), mongopher.WithUpsert())
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 upserted document, got %d", count)
	}
}

func TestFindOneAndUpdate_ReturnBefore(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	doc, err := c.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(doc, &result)
	// default: returns document before the update
	if result["age"] != float64(30) {
		t.Fatalf("expected age=30 (before update), got %v", result["age"])
	}
}

func TestFindOneAndUpdate_ReturnAfter(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	doc, err := c.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":31}`)), mongopher.WithReturnAfter())
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["age"] != float64(31) {
		t.Fatalf("expected age=31 (after update), got %v", result["age"])
	}
}

func TestFindOneAndUpdate_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	_, err := c.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":1}`)))
	if !errors.Is(err, mongopher.ErrNoDocuments) {
		t.Fatalf("expected ErrNoDocuments, got %v", err)
	}
}

func TestFindOneAndUpdate_InvalidJSON(t *testing.T) {
	c := col(t)
	_, err := c.FindOneAndUpdate(context.Background(), mongopher.EmptyFilter(), []byte(`not json`))
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestFindOneAndDelete(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`)); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	doc, err := c.FindOneAndDelete(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]any
	json.Unmarshal(doc, &result)
	if result["name"] != "Alice" {
		t.Fatalf("expected returned doc to be Alice, got %v", result["name"])
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 documents after delete, got %d", count)
	}
}

func TestFindOneAndDelete_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	_, err := c.FindOneAndDelete(ctx, filter)
	if !errors.Is(err, mongopher.ErrNoDocuments) {
		t.Fatalf("expected ErrNoDocuments, got %v", err)
	}
}

// recordingCollection wraps a Collection and counts InsertOne calls.
// This demonstrates the wrapper pattern enabled by the Collection interface.
type recordingCollection struct {
	mongopher.Collection
	insertCount int
}

func (r *recordingCollection) InsertOne(ctx context.Context, doc []byte) (mongopher.InsertResult, error) {
	r.insertCount++
	return r.Collection.InsertOne(ctx, doc)
}

func TestCollection_CanBeWrapped(t *testing.T) {
	ctx := context.Background()
	base := testClient.Collection(t.Name())
	t.Cleanup(func() { _ = base.Drop(context.Background()) })

	rec := &recordingCollection{Collection: base}

	// Overridden method is intercepted
	if _, err := rec.InsertOne(ctx, []byte(`{"name":"Alice"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := rec.InsertOne(ctx, []byte(`{"name":"Bob"}`)); err != nil {
		t.Fatal(err)
	}
	if rec.insertCount != 2 {
		t.Fatalf("expected insertCount=2, got %d", rec.insertCount)
	}

	// Non-overridden methods delegate to the underlying collection
	count, err := rec.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 documents, got %d", count)
	}

	// Wrapper satisfies the Collection interface
	var _ mongopher.Collection = rec
}

func TestCollection_InterfaceReturnedByClient(t *testing.T) {
	// Verify that client.Collection() returns a value assignable to the interface.
	// This is a compile-time guarantee but worth asserting explicitly.
	var _ mongopher.Collection = testClient.Collection("any")
}

func TestBulkUpdate(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"name":"Alice","score":10},{"name":"Bob","score":10},{"name":"Carol","score":10}]`)); err != nil {
		t.Fatal(err)
	}

	filterAlice, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	filterBob, _ := mongopher.FilterFromJSON([]byte(`{"name":"Bob"}`))

	res, err := c.BulkUpdate(ctx, []mongopher.UpdateSpec{
		{Filter: filterAlice, Update: mongopher.Set([]byte(`{"score":99}`))},
		{Filter: filterBob, Update: mongopher.Set([]byte(`{"score":88}`))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 2 {
		t.Fatalf("expected MatchedCount=2, got %d", res.MatchedCount)
	}
	if res.ModifiedCount != 2 {
		t.Fatalf("expected ModifiedCount=2, got %d", res.ModifiedCount)
	}

	doc, _ := c.FindOne(ctx, filterAlice)
	var alice map[string]any
	json.Unmarshal(doc, &alice)
	if alice["score"] != float64(99) {
		t.Fatalf("expected Alice score=99, got %v", alice["score"])
	}

	doc, _ = c.FindOne(ctx, filterBob)
	var bob map[string]any
	json.Unmarshal(doc, &bob)
	if bob["score"] != float64(88) {
		t.Fatalf("expected Bob score=88, got %v", bob["score"])
	}

	// Carol should be untouched
	filterCarol, _ := mongopher.FilterFromJSON([]byte(`{"name":"Carol"}`))
	doc, _ = c.FindOne(ctx, filterCarol)
	var carol map[string]any
	json.Unmarshal(doc, &carol)
	if carol["score"] != float64(10) {
		t.Fatalf("expected Carol score=10 (untouched), got %v", carol["score"])
	}
}

func TestBulkUpdate_InvalidJSON(t *testing.T) {
	c := col(t)
	_, err := c.BulkUpdate(context.Background(), []mongopher.UpdateSpec{
		{Filter: mongopher.EmptyFilter(), Update: []byte(`not json`)},
	})
	if !errors.Is(err, mongopher.ErrInvalidJSON) {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}

func TestBulkUpdate_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	res, err := c.BulkUpdate(ctx, []mongopher.UpdateSpec{
		{Filter: filter, Update: []byte(`{"$set":{"score":99}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 0 {
		t.Fatalf("expected MatchedCount=0, got %d", res.MatchedCount)
	}
}

func TestBulkDelete(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	if _, err := c.InsertMany(ctx, []byte(`[{"name":"Alice"},{"name":"Bob"},{"name":"Carol"}]`)); err != nil {
		t.Fatal(err)
	}

	filterAlice, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
	filterBob, _ := mongopher.FilterFromJSON([]byte(`{"name":"Bob"}`))

	res, err := c.BulkDelete(ctx, []mongopher.Filter{filterAlice, filterBob})
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedCount != 2 {
		t.Fatalf("expected DeletedCount=2, got %d", res.DeletedCount)
	}

	count, err := c.CountDocuments(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining document, got %d", count)
	}
}

func TestBulkDelete_NoMatch(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"nobody"}`))
	res, err := c.BulkDelete(ctx, []mongopher.Filter{filter})
	if err != nil {
		t.Fatal(err)
	}
	if res.DeletedCount != 0 {
		t.Fatalf("expected DeletedCount=0, got %d", res.DeletedCount)
	}
}

func TestDriver_Ping(t *testing.T) {
	ctx := context.Background()
	raw := testClient.Driver()
	res := raw.Database("admin").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}})
	if err := res.Err(); err != nil {
		t.Fatalf("ping via Driver() failed: %v", err)
	}
}
