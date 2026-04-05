package mongopher_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jakubtomas-cz/mongopher"
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

func col(t *testing.T) *mongopher.Collection {
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
	docs := [][]byte{
		[]byte(`{"name":"Alice"}`),
		[]byte(`{"name":"Bob"}`),
		[]byte(`{"name":"Carol"}`),
	}
	res, err := c.InsertMany(context.Background(), docs)
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

	docs := [][]byte{
		[]byte(`{"role":"admin","name":"Alice"}`),
		[]byte(`{"role":"admin","name":"Bob"}`),
		[]byte(`{"role":"user","name":"Carol"}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"admin"}`))
	results, err := c.Find(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
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
	res, err := c.UpdateOne(ctx, filter, []byte(`{"$set":{"age":31}}`))
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

	docs := [][]byte{
		[]byte(`{"role":"admin","score":10}`),
		[]byte(`{"role":"admin","score":10}`),
		[]byte(`{"role":"user","score":10}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	filter, _ := mongopher.FilterFromJSON([]byte(`{"role":"admin"}`))
	res, err := c.UpdateMany(ctx, filter, []byte(`{"$set":{"score":99}}`))
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
	for _, doc := range results {
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

	docs := [][]byte{
		[]byte(`{"role":"admin"}`),
		[]byte(`{"role":"admin"}`),
		[]byte(`{"role":"user"}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
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

	docs := [][]byte{
		[]byte(`{"type":"a"}`),
		[]byte(`{"type":"a"}`),
		[]byte(`{"type":"b"}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
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
	if len(results) != 0 {
		t.Fatalf("expected empty slice, got %d results", len(results))
	}
}

func TestFind_EmptyFilter(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	docs := [][]byte{
		[]byte(`{"n":1}`),
		[]byte(`{"n":2}`),
		[]byte(`{"n":3}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
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
	if len(results) != 3 {
		t.Fatalf("expected 3 results with limit, got %d", len(results))
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
	if len(results) != 2 {
		t.Fatalf("expected 2 results after skip, got %d", len(results))
	}
}

func TestFind_WithSort(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	docs := [][]byte{
		[]byte(`{"score":3}`),
		[]byte(`{"score":1}`),
		[]byte(`{"score":2}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	// Ascending
	results, err := c.Find(ctx, mongopher.EmptyFilter(), mongopher.WithSort("score", mongopher.ASC))
	if err != nil {
		t.Fatal(err)
	}
	scores := make([]float64, len(results))
	for i, doc := range results {
		var m map[string]any
		json.Unmarshal(doc, &m)
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
	for i, doc := range results {
		var m map[string]any
		json.Unmarshal(doc, &m)
		scores[i] = m["score"].(float64)
	}
	if scores[0] != 3 || scores[1] != 2 || scores[2] != 1 {
		t.Fatalf("expected descending order [3,2,1], got %v", scores)
	}
}

func TestFind_WithSort_MultiField(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	docs := [][]byte{
		[]byte(`{"role":"admin","name":"Charlie"}`),
		[]byte(`{"role":"admin","name":"Alice"}`),
		[]byte(`{"role":"user","name":"Bob"}`),
		[]byte(`{"role":"user","name":"Alice"}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
		t.Fatal(err)
	}

	results, err := c.Find(ctx, mongopher.EmptyFilter(),
		mongopher.WithSort("role", mongopher.ASC), // role ASC
		mongopher.WithSort("name", mongopher.ASC), // then name ASC
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
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
	for i, raw := range results {
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
	res, err := c.UpdateOne(ctx, filter, []byte(`{"$set":{"age":1}}`))
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

	if errors.Is(err, mongopher.ErrTransactionsNotSupported) {
		t.Fatal("regular transaction error must not be ErrTransactionsNotSupported")
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
		_, err := inventory.UpdateOne(ctx, filter, []byte(`{"$inc":{"stock":-2}}`))
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

	docs := [][]byte{[]byte(`{"x":1}`), []byte(`{"x":2}`)}
	if _, err := c.InsertMany(ctx, docs); err != nil {
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
	docs := [][]byte{
		[]byte(`{"name":"Alice"}`),
		[]byte(`not json`),
		[]byte(`{"name":"Carol"}`),
	}
	_, err := c.InsertMany(context.Background(), docs)
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

	docs := [][]byte{
		[]byte(`{"city":"Prague","status":"active"}`),
		[]byte(`{"city":"Prague","status":"active"}`),
		[]byte(`{"city":"Brno","status":"active"}`),
		[]byte(`{"city":"Brno","status":"inactive"}`),
		[]byte(`{"city":"Brno","status":"inactive"}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
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
	if len(results) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(results))
	}

	type group struct {
		ID    string  `json:"_id"`
		Count float64 `json:"count"`
	}
	var first, second group
	json.Unmarshal(results[0], &first)
	json.Unmarshal(results[1], &second)

	if first.ID != "Brno" || first.Count != 3 {
		t.Fatalf("expected Brno=3, got %s=%v", first.ID, first.Count)
	}
	if second.ID != "Prague" || second.Count != 2 {
		t.Fatalf("expected Prague=2, got %s=%v", second.ID, second.Count)
	}
}

func TestAggregate_MatchAndProject(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	docs := [][]byte{
		[]byte(`{"name":"Alice","score":80}`),
		[]byte(`{"name":"Bob","score":40}`),
		[]byte(`{"name":"Carol","score":95}`),
	}
	if _, err := c.InsertMany(ctx, docs); err != nil {
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
	if len(results) != 2 {
		t.Fatalf("expected 2 results after $match, got %d", len(results))
	}
	for _, r := range results {
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
	if results != nil {
		t.Fatalf("expected nil for empty aggregate result, got %v", results)
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

	name, err := c.CreateIndex(ctx, "email", mongopher.ASC)
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

	_, err := c.CreateIndex(ctx, "email", mongopher.ASC, mongopher.WithUnique())
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

	name, err := c.CreateIndex(ctx, "createdAt", mongopher.DESC)
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatal("expected non-empty index name")
	}
}

func TestListIndexes(t *testing.T) {
	ctx := context.Background()
	c := col(t)

	// MongoDB always creates a default _id index
	if _, err := c.InsertOne(ctx, []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateIndex(ctx, "email", mongopher.ASC, mongopher.WithUnique()); err != nil {
		t.Fatal(err)
	}

	indexes, err := c.ListIndexes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Expect at least the _id index and our new one
	if len(indexes) < 2 {
		t.Fatalf("expected at least 2 indexes, got %d", len(indexes))
	}
	for _, idx := range indexes {
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

	name, err := c.CreateIndex(ctx, "email", mongopher.ASC)
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
	if len(after) != len(before)-1 {
		t.Fatalf("expected %d indexes after drop, got %d", len(before)-1, len(after))
	}
}
