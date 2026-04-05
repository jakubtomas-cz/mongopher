// Package mongopher provides a thin, JSON-native MongoDB access layer.
//
// The design goal is simplicity: pass JSON in, get JSON back.
// No struct tags, no code generation, no ORM ceremony.
//
// mongopher builds on the official MongoDB Go driver
// (go.mongodb.org/mongo-driver/v2), which is pulled in automatically
// as a transitive dependency.
//
// # Connecting
//
// Connect dials MongoDB and pings the server to verify connectivity.
// Always defer Disconnect to release the underlying connection pool.
//
//	client, err := mongopher.Connect(ctx, "mongodb://localhost:27017", "mydb")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Disconnect(ctx)
//
//	col := client.Collection("users")
//
// Optional driver options can be passed as extra arguments and are merged
// after the base URI options. This covers TLS, authentication, timeouts,
// and anything else exposed by go.mongodb.org/mongo-driver/v2/mongo/options.
//
//	tlsOpt := options.Client().SetTLSConfig(tlsCfg)
//	client, err := mongopher.Connect(ctx, uri, "mydb", tlsOpt)
//
// TLS can also be enabled via the URI without any extra arguments:
//
//	client, err := mongopher.Connect(ctx, "mongodb://host:27017/?tls=true", "mydb")
//
// # Inserting documents
//
//	res, err := col.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`))
//	fmt.Println(res.InsertedID) // hex ObjectID string, e.g. "507f1f77bcf86cd799439011"
//
//	many, err := col.InsertMany(ctx, [][]byte{
//	    []byte(`{"name":"Bob"}`),
//	    []byte(`{"name":"Carol"}`),
//	})
//	fmt.Println(many.InsertedIDs) // []string of hex IDs
//
// To insert from a struct or map, marshal it with encoding/json first:
//
//	data, _ := json.Marshal(User{Name: "Alice", Age: 30})
//	res, err := col.InsertOne(ctx, data)
//
//	data, _ = json.Marshal(map[string]any{"name": "Alice", "age": 30})
//	res, err = col.InsertOne(ctx, data)
//
// # Querying
//
// Filters are built from JSON and passed to all read/write/delete operations.
// Use EmptyFilter to match all documents.
//
//	filter, err := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
//
//	doc, err := col.FindOne(ctx, filter)  // returns []byte JSON, or ErrNoDocuments
//
//	docs, err := col.Find(ctx, mongopher.EmptyFilter(),
//	    mongopher.WithLimit(10),
//	    mongopher.WithSkip(0),
//	    mongopher.WithSort("name", mongopher.ASC),
//	)
//	for _, d := range docs {
//	    fmt.Println(string(d))
//	}
//
// Find returns nil (not an error) when no documents match. Both len(docs) == 0
// and range docs are safe.
//
// WithSort can be applied multiple times for multi-field sorting:
//
//	col.Find(ctx, filter,
//	    mongopher.WithSort("role", mongopher.ASC),
//	    mongopher.WithSort("createdAt", mongopher.DESC),
//	)
//
// # Updating documents
//
// Update methods accept a standard MongoDB update document (e.g. {"$set":{...}}).
//
//	filter, _ := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
//	update := []byte(`{"$set":{"age":31}}`)
//
//	res, err := col.UpdateOne(ctx, filter, update)
//	fmt.Println(res.MatchedCount, res.ModifiedCount)
//
//	res, err = col.UpdateMany(ctx, filter, update)
//
// If no document matches, err is nil and MatchedCount is 0.
// Check MatchedCount explicitly if you need to detect a no-op update.
//
// # Deleting documents
//
//	res, err := col.DeleteOne(ctx, filter)
//	fmt.Println(res.DeletedCount)
//
//	res, err = col.DeleteMany(ctx, filter)
//
// # Counting and dropping
//
//	n, err := col.CountDocuments(ctx, mongopher.EmptyFilter())
//
//	err = col.Drop(ctx) // removes the entire collection
//
// # Indexes
//
// CreateIndex creates a single-field index and returns its name.
// Use IndexOption values to configure uniqueness, sparseness, or TTL.
//
//	// Simple index
//	name, err := col.CreateIndex(ctx, "email", mongopher.ASC)
//
//	// Unique index
//	name, err := col.CreateIndex(ctx, "email", mongopher.ASC, mongopher.WithUnique())
//
//	// TTL index — documents expire after 3600 seconds
//	name, err := col.CreateIndex(ctx, "createdAt", mongopher.ASC, mongopher.WithTTL(3600))
//
//	// Drop an index by name
//	err = col.DropIndex(ctx, name)
//
//	// List all indexes as JSON documents
//	indexes, err := col.ListIndexes(ctx)
//	for _, idx := range indexes {
//	    fmt.Println(string(idx))
//	}
//
// # Aggregation
//
// Aggregate runs a MongoDB aggregation pipeline. The pipeline is a JSON array
// of stage documents; each stage transforms the documents passing through it.
// Returns nil (not an error) when the pipeline produces no results.
//
//	pipeline := []byte(`[
//	    {"$match": {"status": "active"}},
//	    {"$group": {"_id": "$city", "count": {"$sum": 1}}},
//	    {"$sort": {"count": -1}}
//	]`)
//
//	docs, err := col.Aggregate(ctx, pipeline)
//	for _, doc := range docs {
//	    fmt.Println(string(doc)) // {"_id":"Prague","count":42}
//	}
//
// Common stages: $match (filter), $project (reshape), $group (summarise),
// $sort, $limit, $skip, $lookup (join), $unwind (flatten arrays).
//
// # Transactions
//
// WithTransaction runs fn inside a multi-document ACID transaction.
// The ctx passed to fn must be forwarded to all collection operations
// so they participate in the transaction. Returning a non-nil error
// aborts the transaction; returning nil commits it.
// Requires a replica set or sharded cluster.
//
//	err := client.WithTransaction(ctx, func(ctx context.Context) error {
//	    if _, err := orders.InsertOne(ctx, orderJSON); err != nil {
//	        return err
//	    }
//	    filter, _ := mongopher.FilterFromJSON([]byte(`{"sku":"ABC"}`))
//	    _, err := inventory.UpdateOne(ctx, filter, []byte(`{"$inc":{"stock":-1}}`))
//	    return err
//	})
//
// # _id handling
//
// MongoDB ObjectIDs are returned as plain hex strings, not Extended JSON.
// A document stored without an explicit _id gets one assigned by MongoDB;
// the returned JSON will contain `"_id":"507f1f77bcf86cd799439011"`.
// You may also supply your own _id in the insert payload.
//
// # Number types
//
// JSON numbers are decoded using standard Go JSON rules, which means all
// numeric values are represented as float64 when stored in untyped structures
// (e.g. map[string]any). Integer values round-trip correctly for normal use.
// If exact integer types matter, unmarshal the returned JSON into a typed
// struct or use json.Number.
//
// # Error handling
//
// Sentinel errors are wrapped, so always use errors.Is for comparison:
//
//	doc, err := col.FindOne(ctx, filter)
//	if errors.Is(err, mongopher.ErrNoDocuments) {
//	    // no match
//	}
//
//	_, err = col.InsertOne(ctx, []byte(`not json`))
//	if errors.Is(err, mongopher.ErrInvalidJSON) {
//	    // malformed input
//	}
package mongopher
