// Package mongopher provides a thin, JSON-native MongoDB access layer.
//
// The design goal is simplicity: pass JSON in, get JSON back.
// No struct tags, no code generation, no ORM ceremony.
//
// mongopher builds on the official MongoDB Go driver
// (go.mongodb.org/mongo-driver/v2), which is pulled in automatically
// as a transitive dependency.
//
// # Design philosophy
//
// Every method accepts and returns plain []byte JSON. There are no intermediate
// types to construct, no struct tags to maintain, and no marshalling step
// between your data and the database.
//
// A raw HTTP request body can go straight into MongoDB:
//
//	body, _ := io.ReadAll(r.Body)
//	res, err := col.InsertOne(ctx, body)
//
//	body, _ = io.ReadAll(r.Body)
//	res, err = col.UpdateOne(ctx, mongopher.FilterByID(id), mongopher.Set(body))
//
//	body, _ = io.ReadAll(r.Body) // JSON array
//	res, err = col.InsertMany(ctx, body)
//
// And a MongoDB result can go straight back out:
//
//	doc, err := col.FindOne(ctx, mongopher.FilterByID(id))
//	w.Header().Set("Content-Type", "application/json")
//	w.Write(doc)
//
//	docs, err := col.Find(ctx, mongopher.EmptyFilter())
//	w.Write(docs) // already a JSON array
//
// When you do need a typed value, Marshal and UnmarshalAs bridge the gap:
//
//	data, err := mongopher.Marshal(User{Name: "Alice", Age: 30})
//	res, err := col.InsertOne(ctx, data)
//
//	doc, err := col.FindOne(ctx, filter)
//	user, err := mongopher.UnmarshalAs[User](doc)
//
//	docs, err := col.Find(ctx, mongopher.EmptyFilter())
//	users, err := mongopher.UnmarshalAs[[]User](docs)
//
// # Example
//
// The examples/todo-server directory contains a fully working REST API —
// create, list, get, update, and delete todos — backed by MongoDB.
// It demonstrates the zero-ceremony pattern end to end: request bodies go
// straight into the database, query results go straight back out.
// Each handler has curl examples in its godoc comment.
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
//	many, err := col.InsertMany(ctx, []byte(`[{"name":"Bob"},{"name":"Carol"}]`))
//	fmt.Println(many.InsertedIDs) // []string of hex IDs
//
// To insert from a struct or map, use mongopher.Marshal:
//
//	data, err := mongopher.Marshal(User{Name: "Alice", Age: 30})
//	res, err = col.InsertOne(ctx, data)
//
//	data, err = mongopher.Marshal(map[string]any{"name": "Alice", "age": 30})
//	res, err = col.InsertOne(ctx, data)
//
// # Querying
//
// Filters are built with typed helpers or from raw JSON and passed to all
// read/write/delete operations.
//
//	// Typed helpers
//	mongopher.Eq("status", "active")
//	mongopher.Ne("status", "deleted")
//	mongopher.Gt("age", 18)
//	mongopher.Gte("age", 18)
//	mongopher.Lt("age", 65)
//	mongopher.Lte("age", 65)
//	mongopher.In("role", "admin", "owner")
//	mongopher.Exists("deletedAt", false)
//
//	// Combine conditions
//	mongopher.And(mongopher.Eq("status", "active"), mongopher.Gt("age", 18))
//
//	// Match any of several conditions
//	mongopher.Or(mongopher.Eq("role", "admin"), mongopher.Eq("role", "owner"))
//
//	// Raw JSON for anything else ($regex, dot notation, ...)
//	filter, err := mongopher.FilterFromJSON([]byte(`{"name":"Alice"}`))
//
//	// Filter by _id:
//	filter, err := mongopher.FilterByID("user-42")
//
//	// Filters are passed directly to any read, write, or delete operation:
//	doc, err := col.FindOne(ctx, mongopher.Eq("email", "alice@example.com"))
//
//	docs, err := col.Find(ctx, mongopher.And(
//	    mongopher.Eq("status", "active"),
//	    mongopher.Gte("age", 18),
//	))
//
//	docs, err := col.Find(ctx, mongopher.EmptyFilter(),
//	    mongopher.WithLimit(10),
//	    mongopher.WithSkip(0),
//	    mongopher.WithSort("name", mongopher.ASC),
//	)
//	fmt.Println(string(docs))
//	// [{"_id":"507f...","name":"Alice","age":30},{"_id":"...","name":"Bob","age":25}]
//
// Find always returns a valid JSON array. An empty result set is [], never nil.
//
// Use UnmarshalAs to decode results — T is the full result type:
//
//	type User struct { Name string `json:"name"` }
//
//	// Array from Find/Aggregate/ListIndexes
//	users, err := mongopher.UnmarshalAs[[]User](docs)
//	fmt.Println(users[0].Name) // Alice
//
//	// Untyped array
//	items, err := mongopher.UnmarshalAs[[]map[string]any](docs)
//	fmt.Println(items[0]["name"]) // Alice
//
//	// Single document from FindOne/FindOneAndUpdate
//	user, err := mongopher.UnmarshalAs[User](doc)
//	fmt.Println(user.Name) // Alice
//
// WithSort can be applied multiple times for multi-field sorting:
//
//	col.Find(ctx, filter,
//	    mongopher.WithSort("role", mongopher.ASC),
//	    mongopher.WithSort("createdAt", mongopher.DESC),
//	)
//
// WithFields limits the returned fields (like SELECT in SQL). It is accepted
// by both Find and FindOne. _id is always included.
//
//	docs, err := col.Find(ctx, filter, mongopher.WithFields("name", "email"))
//	doc, err := col.FindOne(ctx, filter, mongopher.WithFields("name", "email"))
//
// # Updating documents
//
// Use the update helpers to wrap any JSON object in a MongoDB operator:
//
//	res, err := col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
//	res, err := col.UpdateMany(ctx, filter, mongopher.Inc([]byte(`{"loginCount":1}`)))
//	fmt.Println(res.MatchedCount, res.ModifiedCount)
//
// Available helpers: Set, Unset, Inc, Push, Pull, AddToSet, Rename.
//
// This is especially useful when forwarding a request body — the JSON passes
// straight through without any wrapping ceremony:
//
//	body, _ := io.ReadAll(r.Body)
//	res, err := col.UpdateOne(ctx, mongopher.FilterByID(id), mongopher.Set(body))
//
// For operators without a helper, pass the raw update document directly:
//
//	res, err := col.UpdateOne(ctx, filter, []byte(`{"$bit":{"flags":{"or":4}}}`))
//
// If no document matches, err is nil and MatchedCount is 0.
// Check MatchedCount explicitly if you need to detect a no-op update.
//
// Pass WithUpsert to insert a new document when no match is found:
//
//	res, err := col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"role":"admin"}`)), mongopher.WithUpsert())
//	res, err = col.UpdateMany(ctx, filter, mongopher.Set([]byte(`{"active":true}`)), mongopher.WithUpsert())
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
//	doc, err := col.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
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
//	    {Filter: filterAlice, Update: mongopher.Set([]byte(`{"score":99}`))},
//	    {Filter: filterBob,   Update: mongopher.Set([]byte(`{"score":88}`))},
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
//	// List all indexes — returns a JSON array
//	indexes, err := col.ListIndexes(ctx)
//	fmt.Println(string(indexes))
//	// [{"v":2,"key":{"_id":1},"name":"_id_"},{"v":2,"key":{"email":1},"name":"email_1","unique":true}]
//
// # Aggregation
//
// Aggregate runs a MongoDB aggregation pipeline. The pipeline is a JSON array
// of stage documents; each stage transforms the documents passing through it.
// Always returns a valid JSON array; an empty result is [], never nil.
//
//	pipeline := []byte(`[
//	    {"$match": {"status": "active"}},
//	    {"$group": {"_id": "$city", "count": {"$sum": 1}}},
//	    {"$sort": {"count": -1}}
//	]`)
//
//	docs, err := col.Aggregate(ctx, pipeline)
//	fmt.Println(string(docs))
//	// [{"_id":"Prague","count":42},{"_id":"Berlin","count":31}]
//
// Common stages: $match (filter), $project (reshape), $group (summarise),
// $sort, $limit, $skip, $lookup (join), $unwind (flatten arrays).
//
// # Replica sets and sharded clusters
//
// Change streams and transactions require MongoDB to run as a replica set or
// sharded cluster. A standalone mongod instance is not sufficient.
//
// For local development, a single-node replica set is the simplest option:
//
//	docker run -d --name mongopher-mongo -p 27017:27017 mongo:latest --replSet rs0
//	sleep 2
//	docker exec mongopher-mongo mongosh --eval \
//	    'rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]})'
//
// The included Makefile wraps this as `make mongo-up`.
//
// In production, use a proper multi-node replica set or a managed cluster
// (e.g. MongoDB Atlas), which runs as a replica set by default.
//
// Calling WithTransaction or Watch against a standalone instance returns
// ErrReplicaSetRequired.
//
// # Change streams
//
// Watch opens a change stream on the collection and returns an iterator over
// ChangeEvent values. Change streams require a replica set or sharded cluster.
//
//	cs, err := col.Watch(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cs.Close(ctx)
//
//	for cs.Next(ctx) {
//	    ev, err := cs.Event()
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Println(ev.OperationType, ev.DocumentID)
//	    fmt.Println(string(ev.Document))
//	}
//
// ChangeEvent.OperationType is one of: "insert", "update", "replace",
// "delete", "drop", "invalidate".
// ChangeEvent.Document is nil for delete/drop/invalidate events and for
// update events when WithFullDocument is not set.
// ChangeEvent.DocumentID is empty for non-document events (drop, invalidate).
//
// Watch accepts options:
//
//	// Include the full document on update events
//	cs, err := col.Watch(ctx, mongopher.WithFullDocument())
//
//	// Filter to specific operation types
//	cs, err := col.Watch(ctx, mongopher.WithOperationTypes("insert", "delete"))
//
// cs.Next(ctx) blocks until an event arrives or the context is done —
// cancel the context to stop the stream.
//
// # Transactions
//
// WithTransaction runs fn inside an ACID transaction. It is available on both
// Collection (single-collection convenience) and Client (multi-collection).
// The ctx passed to fn must be forwarded to all collection operations
// so they participate in the transaction. Returning a non-nil error
// aborts the transaction; returning nil commits it.
// Returns ErrReplicaSetRequired on standalone instances.
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
//	    _, err := inventory.UpdateOne(ctx, filter, mongopher.Inc([]byte(`{"stock":-1}`)))
//	    return err
//	})
//	if errors.Is(err, mongopher.ErrReplicaSetRequired) {
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
//	    m, err := mongopher.UnmarshalAs[map[string]any](doc)
//	    if err != nil {
//	        return mongopher.InsertResult{}, err
//	    }
//	    m["createdAt"] = time.Now().UTC()
//	    doc, err = mongopher.Marshal(m)
//	    if err != nil {
//	        return mongopher.InsertResult{}, err
//	    }
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
// # Raw driver access
//
// Client.Driver returns the underlying *mongo.Client for anything not covered
// by mongopher — advanced queries, admin commands, or driver features that
// haven't been wrapped yet:
//
//	raw := client.Driver()
//	raw.Database("mydb").RunCommand(ctx, bson.D{{Key: "ping", Value: 1}})
//	raw.Database("mydb").Collection("users").FindOne(ctx, bson.D{})
//
// # _id handling
//
// MongoDB ObjectIDs are returned as plain hex strings, not Extended JSON.
// A document stored without an explicit _id gets one assigned by MongoDB;
// the returned JSON will contain `"_id":"507f1f77bcf86cd799439011"`.
// You may also supply your own _id in the insert payload.
//
// Filters round-trip correctly — a hex string _id passed to FilterFromJSON
// or FilterByID is automatically coerced back to a BSON ObjectID, so the
// typical fetch-then-filter pattern works without any manual conversion.
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
