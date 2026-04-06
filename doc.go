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
// Pass WithUpsert to insert a new document when no match is found:
//
//	res, err := col.UpdateOne(ctx, filter, update, mongopher.WithUpsert())
//	res, err = col.UpdateMany(ctx, filter, update, mongopher.WithUpsert())
//
// # Replacing documents
//
// ReplaceOne swaps the entire matched document. No update operators — just the
// full replacement document. Fields absent from the replacement are removed.
//
//	res, err := col.ReplaceOne(ctx, filter, []byte(`{"name":"Alice","age":31}`))
//	fmt.Println(res.MatchedCount, res.ModifiedCount)
//
// WithUpsert is also accepted:
//
//	res, err = col.ReplaceOne(ctx, filter, replacement, mongopher.WithUpsert())
//
// # Atomic find-and-modify
//
// FindOneAndUpdate and FindOneAndDelete find a document, apply the change,
// and return the document — all atomically. Both return ErrNoDocuments when
// no document matches the filter.
//
//	// Returns the document before the update (default)
//	doc, err := col.FindOneAndUpdate(ctx, filter, []byte(`{"$set":{"age":31}}`))
//
//	// Returns the document after the update
//	doc, err = col.FindOneAndUpdate(ctx, filter, update, mongopher.WithReturnAfter())
//
//	// Returns the deleted document
//	doc, err = col.FindOneAndDelete(ctx, filter)
//
// # Deleting documents
//
//	res, err := col.DeleteOne(ctx, filter)
//	fmt.Println(res.DeletedCount)
//
//	res, err = col.DeleteMany(ctx, filter)
//
// # Bulk operations
//
// BulkUpdate and BulkDelete send multiple operations to MongoDB in a single
// round-trip. Use InsertMany for bulk inserts.
//
//	res, err := col.BulkUpdate(ctx, []mongopher.UpdateSpec{
//	    {Filter: filterAlice, Update: []byte(`{"$set":{"score":99}}`)},
//	    {Filter: filterBob,   Update: []byte(`{"$set":{"score":88}}`)},
//	})
//	fmt.Println(res.MatchedCount, res.ModifiedCount)
//
//	res, err = col.BulkDelete(ctx, []mongopher.Filter{filterAlice, filterBob})
//	fmt.Println(res.DeletedCount)
//
// Bulk operations are ordered but not transactional. If one operation fails,
// MongoDB stops processing the remaining ones but does not roll back those
// that already succeeded. Wrap in WithTransaction if you need all-or-nothing
// behaviour.
//
// # Counting and dropping
//
//	n, err := col.CountDocuments(ctx, mongopher.EmptyFilter())
//
//	err = col.Drop(ctx) // removes the entire collection
//
// # Indexes
//
// CreateIndex creates an index on one or more fields and returns its name.
// Pass a single IndexKey for a single-field index, multiple for a compound index.
// Use IndexOption values to configure uniqueness, sparseness, or TTL.
//
//	// Single-field index
//	name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
//	    {Field: "email", Direction: mongopher.ASC},
//	})
//
//	// Compound index
//	name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
//	    {Field: "role", Direction: mongopher.ASC},
//	    {Field: "createdAt", Direction: mongopher.DESC},
//	})
//
//	// Unique index
//	name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
//	    {Field: "email", Direction: mongopher.ASC},
//	}, mongopher.WithUnique())
//
//	// TTL index — documents expire after 3600 seconds
//	name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
//	    {Field: "createdAt", Direction: mongopher.ASC},
//	}, mongopher.WithTTL(3600))
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
// WithTransaction runs fn inside an ACID transaction. It is available on both
// Collection (single-collection convenience) and Client (multi-collection).
// The ctx passed to fn must be forwarded to all collection operations
// so they participate in the transaction. Returning a non-nil error
// aborts the transaction; returning nil commits it.
// Returns ErrTransactionsNotSupported on standalone instances.
//
// Single-collection transaction via Collection:
//
//	err := col.WithTransaction(ctx, func(ctx context.Context) error {
//	    _, err := col.InsertOne(ctx, docJSON)
//	    return err
//	})
//
// col.WithTransaction also works for multi-collection transactions — what ties
// operations to a transaction is the ctx, not which object WithTransaction is
// called on. client.WithTransaction is simply more explicit about the intent.
//
// Multi-collection transaction via Client:
//
//	err := client.WithTransaction(ctx, func(ctx context.Context) error {
//	    if _, err := orders.InsertOne(ctx, orderJSON); err != nil {
//	        return err
//	    }
//	    filter, _ := mongopher.FilterFromJSON([]byte(`{"sku":"ABC"}`))
//	    _, err := inventory.UpdateOne(ctx, filter, []byte(`{"$inc":{"stock":-1}}`))
//	    return err
//	})
//	if errors.Is(err, mongopher.ErrTransactionsNotSupported) {
//	    // instance is not a replica set or sharded cluster
//	}
//
// # Extending Collection
//
// Collection is an interface, so you can wrap it to intercept or augment any
// operation without modifying the library. Embed mongopher.Collection in your
// own struct and override only the methods you care about — the rest delegate
// to the underlying implementation automatically.
//
//	type TimestampedCollection struct {
//	    mongopher.Collection
//	}
//
//	func (c *TimestampedCollection) InsertOne(ctx context.Context, doc []byte) (mongopher.InsertResult, error) {
//	    var m map[string]any
//	    if err := json.Unmarshal(doc, &m); err != nil {
//	        return mongopher.InsertResult{}, err
//	    }
//	    m["createdAt"] = time.Now().UTC()
//	    doc, _ = json.Marshal(m)
//	    return c.Collection.InsertOne(ctx, doc)
//	}
//
// Use it anywhere a Collection is expected:
//
//	col := &TimestampedCollection{Collection: client.Collection("users")}
//	col.InsertOne(ctx, []byte(`{"name":"Alice"}`)) // createdAt added automatically
//	col.FindOne(ctx, filter)                        // delegates to the underlying collection
//
// Common use cases: automatic timestamps, audit logging, input validation,
// cache invalidation, instrumentation.
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
