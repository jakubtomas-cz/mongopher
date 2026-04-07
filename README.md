# mongopher

A thin, JSON-native MongoDB access layer for Go.

Pass JSON in, get JSON back. No struct tags, no code generation, no ORM ceremony — just a clean bridge between your JSON data and MongoDB.

## Features

- JSON-native: no struct tags, no codegen, no ORM
- CRUD, aggregation, indexes, and transactions out of the box
- Atomic find-and-modify: `FindOneAndUpdate`, `FindOneAndDelete`
- Full document replacement with `ReplaceOne`
- Upsert support on `UpdateOne`, `UpdateMany`, and `ReplaceOne`
- Typed filter helpers (`Eq`, `Ne`, `Gt`, `In`, `Exists`, `And`, ...) with raw JSON fallback
- Update operator helpers (`Set`, `Inc`, `Push`, ...) — wrap any JSON body in a MongoDB operator, no string construction needed
- Sorting, pagination, and multi-field ordering
- ObjectIDs as plain hex strings — no Extended JSON noise
- Change streams for real-time insert/update/delete notifications
- Thin wrapper over the official MongoDB Go driver — no magic, full driver access when needed
- Thoroughly unit tested against a real MongoDB instance

## Installation

```bash
go get github.com/jakubtomas-cz/mongopher
```

Requires Go 1.25+ and a running MongoDB instance.

mongopher builds on the [official MongoDB Go driver](https://github.com/mongodb/mongo-go-driver) (`go.mongodb.org/mongo-driver/v2`), which is pulled in automatically as a dependency.

## Quick start

```go
ctx := context.Background()

client, err := mongopher.Connect(ctx, "mongodb://localhost:27017", "mydb")
if err != nil {
    log.Fatal(err)
}
defer client.Disconnect(ctx)

users := client.Collection("users")

// Insert
res, err := users.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`))
fmt.Println(res.InsertedID) // "507f1f77bcf86cd799439011"

// Find
doc, err := users.FindOne(ctx, mongopher.Eq("name", "Alice"))
fmt.Println(string(doc)) // {"_id":"507f1f77...","name":"Alice","age":30}
```

## Connecting

```go
client, err := mongopher.Connect(ctx, uri, databaseName)
```

`Connect` establishes a connection and pings the server before returning, so any connectivity errors surface immediately.

```go
defer client.Disconnect(ctx)
```

### Driver options

An optional variadic argument accepts any `*options.ClientOptions` from the official driver, merged after the base URI options. This covers TLS, authentication, timeouts, and anything else the driver exposes.

```go
import "go.mongodb.org/mongo-driver/v2/mongo/options"

// TLS with a custom CA
tlsOpt := options.Client().SetTLSConfig(tlsCfg)
client, err := mongopher.Connect(ctx, uri, "mydb", tlsOpt)

// Multiple options
client, err := mongopher.Connect(ctx, uri, "mydb",
    options.Client().SetTLSConfig(tlsCfg),
    options.Client().SetServerSelectionTimeout(5*time.Second),
)
```

The simplest way to enable TLS without a custom certificate is via the URI itself:

```
mongodb://user:pass@host:27017/?tls=true
```

## Filters

Filters are built with typed helpers or from raw JSON.

```go
// Equality
mongopher.Eq("status", "active")

// Comparisons
mongopher.Ne("status", "deleted")
mongopher.Gt("age", 18)
mongopher.Gte("age", 18)
mongopher.Lt("age", 65)
mongopher.Lte("age", 65)

// Membership
mongopher.In("role", "admin", "owner")

// Field presence
mongopher.Exists("deletedAt", false) // documents without the field
mongopher.Exists("deletedAt", true)  // documents with the field

// Combine with And
mongopher.And(
    mongopher.Eq("status", "active"),
    mongopher.Gt("age", 18),
)

// Match all documents
mongopher.EmptyFilter()

// Match by _id
filter, err := mongopher.FilterByID("user-42")
```

Filters are passed directly to any read, write, or delete operation:

```go
doc, err := col.FindOne(ctx, mongopher.Eq("email", "alice@example.com"))

docs, err := col.Find(ctx, mongopher.And(
    mongopher.Eq("status", "active"),
    mongopher.Gte("age", 18),
))

res, err := col.UpdateMany(ctx, mongopher.In("role", "admin", "owner"), mongopher.Set([]byte(`{"reviewed":true}`)))

res, err := col.DeleteMany(ctx, mongopher.Exists("deletedAt", true))
```

For anything not covered by the helpers — `$or`, `$regex`, dot notation, nested operators — fall back to raw JSON:

```go
filter, err := mongopher.FilterFromJSON([]byte(`{"address.city":"Prague"}`))
filter, err := mongopher.FilterFromJSON([]byte(`{"$or":[{"role":"admin"},{"role":"owner"}]}`))
```

## CRUD operations

### Insert

```go
// Single document
res, err := col.InsertOne(ctx, []byte(`{"name":"Alice","age":30}`))
fmt.Println(res.InsertedID) // plain hex string

// Multiple documents
res, err := col.InsertMany(ctx, [][]byte{
    []byte(`{"name":"Alice"}`),
    []byte(`{"name":"Bob"}`),
})
fmt.Println(res.InsertedIDs) // []string{"507f...", "507f..."}
```

If a document does not contain `_id`, MongoDB generates one automatically. You can also provide your own:

```go
col.InsertOne(ctx, []byte(`{"_id":"my-custom-id","name":"Alice"}`))
```

#### Inserting from a struct or map

mongopher accepts `[]byte` JSON, so use `encoding/json` to marshal your existing types before passing them in:

```go
type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

data, err := json.Marshal(User{Name: "Alice", Age: 30})
res, err := col.InsertOne(ctx, data)
```

The same works with `map[string]any`:

```go
data, err := json.Marshal(map[string]any{"name": "Alice", "age": 30})
res, err := col.InsertOne(ctx, data)
```

#### Number types

mongopher uses standard JSON unmarshalling internally, which represents all JSON numbers as `float64`. Integer values round-trip correctly for normal use, but if you read a document back and unmarshal it into a `map[string]any`, numeric fields will be `float64`. Use `json.Number` or a typed struct when exact integer types matter.

### Find

```go
// Single document — returns ErrNoDocuments if nothing matches
doc, err := col.FindOne(ctx, filter)

// All matching documents
docs, err := col.Find(ctx, filter)
for _, doc := range docs {
    fmt.Println(string(doc))
}
```

`Find` returns `nil` when there are no matching documents (not an error). Both `len(docs) == 0` and `range docs` are safe to use.

`Find` accepts optional modifiers:

```go
docs, err := col.Find(ctx, filter,
    mongopher.WithLimit(10),
    mongopher.WithSkip(20),
    mongopher.WithSort("createdAt", mongopher.DESC),
)
```

`WithSort` can be applied multiple times for multi-field sorting:

```go
docs, err := col.Find(ctx, filter,
    mongopher.WithSort("role", mongopher.ASC),
    mongopher.WithSort("createdAt", mongopher.DESC),
)
```

### Update

Use the update helpers to wrap any JSON object in a MongoDB operator — no manual string construction needed:

```go
res, err := col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))
res, err := col.UpdateMany(ctx, filter, mongopher.Inc([]byte(`{"loginCount":1}`)))
fmt.Println(res.MatchedCount, res.ModifiedCount)
```

Available helpers: `Set`, `Unset`, `Inc`, `Push`, `Pull`, `AddToSet`, `Rename`.

This pattern is especially useful when the JSON comes from an HTTP request body — it passes straight through without any wrapping ceremony:

```go
body, _ := io.ReadAll(r.Body)
res, err := col.UpdateOne(ctx, mongopher.FilterByID(id), mongopher.Set(body))
```

If you need an operator that has no helper, pass the raw update document directly:

```go
res, err := col.UpdateOne(ctx, filter, []byte(`{"$bit":{"flags":{"or":4}}}`))
```

If no document matches, `err` is `nil` and `MatchedCount` will be `0`. No error is returned for a no-op update — check `MatchedCount` explicitly if you need to detect that case.

#### Upsert

Pass `WithUpsert()` to insert a new document when no match is found:

```go
res, err := col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"role":"admin"}`)), mongopher.WithUpsert())
res, err := col.UpdateMany(ctx, filter, mongopher.Set([]byte(`{"active":true}`)), mongopher.WithUpsert())
```

### Replace

`ReplaceOne` swaps the entire matched document for a new one (no update operators — just the replacement document):

```go
res, err := col.ReplaceOne(ctx, filter, []byte(`{"name":"Alice","age":31}`))
fmt.Println(res.MatchedCount, res.ModifiedCount)
```

Fields that existed in the original but are absent from the replacement are removed. `WithUpsert()` is also accepted:

```go
res, err := col.ReplaceOne(ctx, filter, []byte(`{"name":"Alice","age":31}`), mongopher.WithUpsert())
```

### Atomic find-and-modify

`FindOneAndUpdate` and `FindOneAndDelete` find a document, apply the change, and return the document — all atomically.

```go
// Returns the document before the update (default)
doc, err := col.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":31}`)))

// Returns the document after the update
doc, err := col.FindOneAndUpdate(ctx, filter, mongopher.Set([]byte(`{"age":31}`)), mongopher.WithReturnAfter())

// Returns the deleted document
doc, err := col.FindOneAndDelete(ctx, filter)
```

Both return `ErrNoDocuments` when no document matches the filter.

### Delete

```go
// Delete the first matching document
res, err := col.DeleteOne(ctx, filter)
fmt.Println(res.DeletedCount)

// Delete all matching documents
res, err := col.DeleteMany(ctx, filter)
```

### Bulk operations

`BulkUpdate` and `BulkDelete` send multiple operations to MongoDB in a single round-trip. Use `InsertMany` for bulk inserts.

```go
// Update multiple documents, each with its own filter
res, err := col.BulkUpdate(ctx, []mongopher.UpdateSpec{
    {Filter: filterAlice, Update: mongopher.Set([]byte(`{"score":99}`))},
    {Filter: filterBob,   Update: mongopher.Set([]byte(`{"score":88}`))},
})
fmt.Println(res.MatchedCount, res.ModifiedCount)

// Delete multiple documents, each with its own filter
res, err := col.BulkDelete(ctx, []mongopher.Filter{filterAlice, filterBob})
fmt.Println(res.DeletedCount)
```

> **Note:** Bulk operations are ordered but not transactional. If one operation fails, MongoDB stops processing the remaining ones but does **not** roll back those that already succeeded. Wrap in `WithTransaction` if you need all-or-nothing behaviour.

### Count

```go
count, err := col.CountDocuments(ctx, filter)

// Count all documents
total, err := col.CountDocuments(ctx, mongopher.EmptyFilter())
```

### Drop

```go
err := col.Drop(ctx)
```

Permanently removes the collection and all its documents. This operation is irreversible.

## Indexes

`CreateIndex` accepts one or more `IndexKey` values — a single key for a single-field index, multiple for a compound index.

```go
// Single-field index
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "email", Direction: mongopher.ASC},
})

// Unique index
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "email", Direction: mongopher.ASC},
}, mongopher.WithUnique())

// Compound index
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "role", Direction: mongopher.ASC},
    {Field: "createdAt", Direction: mongopher.DESC},
})

// Compound unique index
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "org", Direction: mongopher.ASC},
    {Field: "email", Direction: mongopher.ASC},
}, mongopher.WithUnique())

// TTL index — documents expire after 3600 seconds
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "createdAt", Direction: mongopher.ASC},
}, mongopher.WithTTL(3600))

// Sparse index — skips documents missing the field
name, err := col.CreateIndex(ctx, []mongopher.IndexKey{
    {Field: "phone", Direction: mongopher.ASC},
}, mongopher.WithSparse())

// Drop an index by name
err = col.DropIndex(ctx, name)

// List all indexes
indexes, err := col.ListIndexes(ctx)
for _, idx := range indexes {
    fmt.Println(string(idx))
}
```

## Aggregation

`Aggregate` runs a MongoDB aggregation pipeline and returns the result documents as JSON. A pipeline is a JSON array of stage documents — each stage transforms the documents passing through it.

```go
pipeline := []byte(`[
    {"$match": {"status": "active"}},
    {"$group": {"_id": "$city", "count": {"$sum": 1}}},
    {"$sort": {"count": -1}}
]`)

docs, err := col.Aggregate(ctx, pipeline)
for _, doc := range docs {
    fmt.Println(string(doc)) // {"_id":"Prague","count":42}
}
```

`Aggregate` returns `nil` (not an error) when the pipeline produces no results.

### Common stages

| Stage | What it does |
|---|---|
| `$match` | Filters documents — like a `Find` filter |
| `$project` | Reshapes documents: `1` includes a field, `0` excludes it |
| `$group` | Collapses many documents into fewer, computing summaries (`$sum`, `$avg`, `$min`, `$max`, `$push`) |
| `$sort` | Orders results |
| `$limit` / `$skip` | Pagination |
| `$lookup` | Joins another collection |
| `$unwind` | Flattens an array field into one document per element |

## Transactions

`WithTransaction` runs a function inside an ACID transaction and is available on both `Collection` and `Client`. The `ctx` received in the callback must be forwarded to all collection operations so they participate in the same transaction. Returning a non-nil error aborts; returning nil commits.

**Single-collection** — call it on the collection directly:

```go
err := col.WithTransaction(ctx, func(ctx context.Context) error {
    _, err := col.InsertOne(ctx, docJSON)
    return err
})
```

**Multi-collection** — call it on the client and pass the ctx to each collection:

```go
err := client.WithTransaction(ctx, func(ctx context.Context) error {
    if _, err := orders.InsertOne(ctx, orderJSON); err != nil {
        return err // triggers rollback
    }
    filter, _ := mongopher.FilterFromJSON([]byte(`{"sku":"ABC"}`))
    _, err := inventory.UpdateOne(ctx, filter, mongopher.Inc([]byte(`{"stock":-1}`)))
    return err
})
```

> **Note:** `col.WithTransaction` works for multi-collection transactions too — what ties operations to a transaction is the `ctx`, not which object you call `WithTransaction` on. `client.WithTransaction` is simply more explicit about the intent.

Transactions require a replica set or sharded cluster. On a standalone instance, `WithTransaction` returns `ErrReplicaSetRequired`:

```go
err := col.WithTransaction(ctx, fn)
if errors.Is(err, mongopher.ErrReplicaSetRequired) {
    // instance is not a replica set or sharded cluster
}
```

## Extending Collection

`Collection` is an interface, so you can wrap it to intercept or augment any operation without modifying the library. Embed `mongopher.Collection` in your own struct and override only the methods you care about — the rest delegate automatically.

```go
type TimestampedCollection struct {
    mongopher.Collection
}

func (c *TimestampedCollection) InsertOne(ctx context.Context, doc []byte) (mongopher.InsertResult, error) {
    var m map[string]any
    if err := json.Unmarshal(doc, &m); err != nil {
        return mongopher.InsertResult{}, err
    }
    m["createdAt"] = time.Now().UTC()
    doc, _ = json.Marshal(m)
    return c.Collection.InsertOne(ctx, doc)
}
```

Use it anywhere a `Collection` is expected:

```go
col := &TimestampedCollection{Collection: client.Collection("users")}
col.InsertOne(ctx, []byte(`{"name":"Alice"}`)) // createdAt added automatically
col.FindOne(ctx, filter)                        // delegates to the underlying collection
```

Common use cases: automatic timestamps, audit logging, input validation, cache invalidation, instrumentation.

## Replica sets and sharded clusters

Some features — **change streams** and **transactions** — require MongoDB to run as a replica set or sharded cluster. A standalone `mongod` is not sufficient.

For local development and testing, the simplest option is a single-node replica set:

```bash
# Start a single-node replica set via Docker (what make mongo-up does)
docker run -d --name mongopher-mongo -p 27017:27017 mongo:latest --replSet rs0
sleep 2
docker exec mongopher-mongo mongosh --eval 'rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]})'
```

Or just use the included Makefile target:

```bash
make mongo-up
```

In production, use a proper multi-node replica set or a managed cluster (e.g. MongoDB Atlas) which runs as a replica set by default.

When replica set features are called against a standalone instance, both `WithTransaction` and `Watch` return `ErrReplicaSetRequired`.

## Change streams

`Watch` opens a change stream on a collection and returns an iterator over `ChangeEvent` values. Change streams require a replica set or sharded cluster.

```go
cs, err := col.Watch(ctx)
if err != nil {
    log.Fatal(err)
}
defer cs.Close(ctx)

for cs.Next(ctx) {
    ev, err := cs.Event()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(ev.OperationType, ev.DocumentID)
    fmt.Println(string(ev.Document))
}
```

`ChangeEvent` fields:

| Field | Description |
|---|---|
| `OperationType` | `"insert"`, `"update"`, `"replace"`, `"delete"`, `"drop"`, `"invalidate"` |
| `DocumentID` | Hex ObjectID of the affected document; empty for non-document events (`drop`, `invalidate`) |
| `Document` | Full document JSON; `nil` for `delete`/`drop`/`invalidate` events and for `update` events without `WithFullDocument` |

`Watch` accepts options:

```go
// Include the full document on update events (default: only metadata is sent)
cs, err := col.Watch(ctx, mongopher.WithFullDocument())

// Filter to specific operation types
cs, err := col.Watch(ctx, mongopher.WithOperationTypes("insert", "delete"))

// Combine options
cs, err := col.Watch(ctx,
    mongopher.WithFullDocument(),
    mongopher.WithOperationTypes("insert", "update", "replace"),
)
```

`cs.Next(ctx)` blocks until an event arrives or the context is done — cancel the context to stop the stream.

## The `_id` field

MongoDB ObjectIDs are returned as plain hex strings, not as Extended JSON objects:

```json
{"_id":"507f1f77bcf86cd799439011","name":"Alice"}
```

Filters round-trip correctly — a hex string `_id` is automatically coerced back to an ObjectID when passed to `FilterFromJSON` or `FilterByID`, so the typical fetch-then-filter pattern works without any manual conversion:

```go
doc, _ := col.FindOne(ctx, mongopher.EmptyFilter())
// doc contains {"_id":"507f1f77...","name":"Alice"}

filter, _ := mongopher.FilterFromJSON([]byte(`{"_id":"507f1f77..."}`))
col.UpdateOne(ctx, filter, mongopher.Set([]byte(`{"name":"Bob"}`)))
```

## Error handling

```go
doc, err := col.FindOne(ctx, filter)
if errors.Is(err, mongopher.ErrNoDocuments) {
    // no match
}

_, err = mongopher.FilterFromJSON([]byte(`not json`))
if errors.Is(err, mongopher.ErrInvalidJSON) {
    // bad input
}
```

## Running tests

Tests require a running MongoDB instance on `localhost:27017`.

```bash
# Start MongoDB via Docker
make mongo-up

# Run tests
make test

# Stop MongoDB
make mongo-down
```

## License

MIT — see [LICENSE](LICENSE).
