package mongopher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection is the interface for all collection operations.
type Collection interface {
	InsertOne(ctx context.Context, doc []byte) (InsertResult, error)
	InsertMany(ctx context.Context, docs []byte) (InsertManyResult, error)
	FindOne(ctx context.Context, filter Filter, opts ...FindOneOption) ([]byte, error)
	Find(ctx context.Context, filter Filter, opts ...FindOption) ([]byte, error)
	UpdateOne(ctx context.Context, filter Filter, update []byte, opts ...UpdateOption) (UpdateResult, error)
	UpdateMany(ctx context.Context, filter Filter, update []byte, opts ...UpdateOption) (UpdateResult, error)
	ReplaceOne(ctx context.Context, filter Filter, replacement []byte, opts ...UpdateOption) (UpdateResult, error)
	FindOneAndUpdate(ctx context.Context, filter Filter, update []byte, opts ...FindOneAndUpdateOption) ([]byte, error)
	FindOneAndDelete(ctx context.Context, filter Filter) ([]byte, error)
	DeleteOne(ctx context.Context, filter Filter) (DeleteResult, error)
	DeleteMany(ctx context.Context, filter Filter) (DeleteResult, error)
	BulkUpdate(ctx context.Context, ops []UpdateSpec) (UpdateResult, error)
	BulkDelete(ctx context.Context, filters []Filter) (DeleteResult, error)
	CountDocuments(ctx context.Context, filter Filter) (int64, error)
	Aggregate(ctx context.Context, pipeline []byte) ([]byte, error)
	CreateIndex(ctx context.Context, keys []IndexKey, opts ...IndexOption) (string, error)
	DropIndex(ctx context.Context, name string) error
	ListIndexes(ctx context.Context) ([]byte, error)
	Drop(ctx context.Context) error
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Watch(ctx context.Context, opts ...WatchOption) (ChangeStream, error)
}

// mongoCollection wraps a mongo.Collection and implements Collection.
type mongoCollection struct {
	inner *mongo.Collection
}

// InsertResult holds the result of a single insert.
type InsertResult struct {
	InsertedID string
}

// InsertManyResult holds the result of a multi-document insert.
type InsertManyResult struct {
	InsertedIDs []string
}

// UpdateResult holds the result of an update operation.
type UpdateResult struct {
	MatchedCount  int64
	ModifiedCount int64
}

// DeleteResult holds the result of a delete operation.
type DeleteResult struct {
	DeletedCount int64
}

// SortDirection specifies the sort order for an index or query.
type SortDirection = bool

const (
	ASC  SortDirection = true
	DESC SortDirection = false
)

// FindOption configures a Find call.
type FindOption interface{ applyFind(*findOptions) }

// FindOneOption configures a FindOne call.
type FindOneOption interface{ applyFindOne(*findOneOptions) }

type findOptions struct {
	limit      int64
	skip       int64
	sort       bson.D
	projection bson.D
}

type findOneOptions struct {
	projection bson.D
}

// FieldsOption limits which fields are returned. It is accepted by both Find and FindOne.
type FieldsOption struct{ proj bson.D }

func (o FieldsOption) applyFind(fo *findOptions)       { fo.projection = o.proj }
func (o FieldsOption) applyFindOne(fo *findOneOptions) { fo.projection = o.proj }

// WithFields returns only the named fields in each document. _id is always included unless
// the collection was queried with an explicit exclusion via the raw driver.
func WithFields(fields ...string) FieldsOption {
	proj := make(bson.D, len(fields))
	for i, f := range fields {
		proj[i] = bson.E{Key: f, Value: 1}
	}
	return FieldsOption{proj: proj}
}

type limitOption struct{ n int64 }

func (o limitOption) applyFind(fo *findOptions) { fo.limit = o.n }

// WithLimit limits the number of documents returned.
func WithLimit(n int64) FindOption { return limitOption{n} }

type skipOption struct{ n int64 }

func (o skipOption) applyFind(fo *findOptions) { fo.skip = o.n }

// WithSkip skips the first n documents.
func WithSkip(n int64) FindOption { return skipOption{n} }

type sortOption struct {
	field string
	asc   SortDirection
}

func (o sortOption) applyFind(fo *findOptions) {
	dir := 1
	if !o.asc {
		dir = -1
	}
	fo.sort = append(fo.sort, bson.E{Key: o.field, Value: dir})
}

// WithSort sorts results by the given field. ascending=true for ASC, false for DESC.
func WithSort(field string, ascending SortDirection) FindOption {
	return sortOption{field: field, asc: ascending}
}

// InsertOne inserts a single JSON document and returns its inserted ID.
func (c *mongoCollection) InsertOne(ctx context.Context, doc []byte) (InsertResult, error) {
	d, err := jsonToBSON(doc)
	if err != nil {
		return InsertResult{}, err
	}
	res, err := c.inner.InsertOne(ctx, d)
	if err != nil {
		return InsertResult{}, err
	}
	return InsertResult{InsertedID: objectIDToString(res.InsertedID)}, nil
}

// InsertMany inserts multiple JSON documents and returns their inserted IDs.
// docs must be a JSON array of document objects, e.g. [{"name":"Alice"},{"name":"Bob"}].
func (c *mongoCollection) InsertMany(ctx context.Context, docs []byte) (InsertManyResult, error) {
	var rawDocs []json.RawMessage
	if err := json.Unmarshal(docs, &rawDocs); err != nil {
		return InsertManyResult{}, fmt.Errorf("%w: %s", ErrInvalidJSON, err)
	}
	bsons := make([]interface{}, len(rawDocs))
	for i, doc := range rawDocs {
		d, err := jsonToBSON(doc)
		if err != nil {
			return InsertManyResult{}, fmt.Errorf("doc[%d]: %w", i, err)
		}
		bsons[i] = d
	}
	res, err := c.inner.InsertMany(ctx, bsons)
	if err != nil {
		return InsertManyResult{}, err
	}
	ids := make([]string, len(res.InsertedIDs))
	for i, id := range res.InsertedIDs {
		ids[i] = objectIDToString(id)
	}
	return InsertManyResult{InsertedIDs: ids}, nil
}

// FindOne returns the first document matching filter as JSON.
// Returns ErrNoDocuments if no document matches.
func (c *mongoCollection) FindOne(ctx context.Context, filter Filter, opts ...FindOneOption) ([]byte, error) {
	fo := &findOneOptions{}
	for _, o := range opts {
		o.applyFindOne(fo)
	}
	mongoOpts := options.FindOne()
	if len(fo.projection) > 0 {
		mongoOpts.SetProjection(fo.projection)
	}
	res := c.inner.FindOne(ctx, filter.raw, mongoOpts)
	var raw bson.D
	if err := res.Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoDocuments
		}
		return nil, err
	}
	return bsonToJSON(raw)
}

// Find returns all documents matching filter as a JSON array.
func (c *mongoCollection) Find(ctx context.Context, filter Filter, opts ...FindOption) ([]byte, error) {
	fo := &findOptions{}
	for _, o := range opts {
		o.applyFind(fo)
	}

	mongoOpts := options.Find()
	if fo.limit > 0 {
		mongoOpts.SetLimit(fo.limit)
	}
	if fo.skip > 0 {
		mongoOpts.SetSkip(fo.skip)
	}
	if len(fo.sort) > 0 {
		mongoOpts.SetSort(fo.sort)
	}
	if len(fo.projection) > 0 {
		mongoOpts.SetProjection(fo.projection)
	}

	cur, err := c.inner.Find(ctx, filter.raw, mongoOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs [][]byte
	for cur.Next(ctx) {
		var raw bson.D
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := bsonToJSON(raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, data)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return joinJSONArray(docs), nil
}

// UpdateOption configures an Update or Replace call.
type UpdateOption func(*updateOptions)

type updateOptions struct {
	upsert bool
}

// WithUpsert inserts a new document if no document matches the filter.
func WithUpsert() UpdateOption { return func(o *updateOptions) { o.upsert = true } }

// UpdateOne updates the first document matching filter using a MongoDB update document (e.g. {"$set":{...}}).
func (c *mongoCollection) UpdateOne(ctx context.Context, filter Filter, update []byte, opts ...UpdateOption) (UpdateResult, error) {
	uo := &updateOptions{}
	for _, o := range opts {
		o(uo)
	}
	u, err := jsonToBSON(update)
	if err != nil {
		return UpdateResult{}, err
	}
	mongoOpts := options.UpdateOne()
	if uo.upsert {
		mongoOpts.SetUpsert(true)
	}
	res, err := c.inner.UpdateOne(ctx, filter.raw, u, mongoOpts)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// UpdateMany updates all documents matching filter.
func (c *mongoCollection) UpdateMany(ctx context.Context, filter Filter, update []byte, opts ...UpdateOption) (UpdateResult, error) {
	uo := &updateOptions{}
	for _, o := range opts {
		o(uo)
	}
	u, err := jsonToBSON(update)
	if err != nil {
		return UpdateResult{}, err
	}
	mongoOpts := options.UpdateMany()
	if uo.upsert {
		mongoOpts.SetUpsert(true)
	}
	res, err := c.inner.UpdateMany(ctx, filter.raw, u, mongoOpts)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// ReplaceOne replaces the first document matching filter with replacement.
func (c *mongoCollection) ReplaceOne(ctx context.Context, filter Filter, replacement []byte, opts ...UpdateOption) (UpdateResult, error) {
	uo := &updateOptions{}
	for _, o := range opts {
		o(uo)
	}
	r, err := jsonToBSON(replacement)
	if err != nil {
		return UpdateResult{}, err
	}
	mongoOpts := options.Replace()
	if uo.upsert {
		mongoOpts.SetUpsert(true)
	}
	res, err := c.inner.ReplaceOne(ctx, filter.raw, r, mongoOpts)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// FindOneAndUpdateOption configures a FindOneAndUpdate call.
type FindOneAndUpdateOption func(*findOneAndUpdateOptions)

type findOneAndUpdateOptions struct {
	returnAfter bool
}

// WithReturnAfter makes FindOneAndUpdate return the document as it looks after the update.
// By default the document before the update is returned.
func WithReturnAfter() FindOneAndUpdateOption {
	return func(o *findOneAndUpdateOptions) { o.returnAfter = true }
}

// FindOneAndUpdate atomically finds the first document matching filter, applies update, and returns it.
// Returns ErrNoDocuments if no document matches.
func (c *mongoCollection) FindOneAndUpdate(ctx context.Context, filter Filter, update []byte, opts ...FindOneAndUpdateOption) ([]byte, error) {
	fo := &findOneAndUpdateOptions{}
	for _, o := range opts {
		o(fo)
	}
	u, err := jsonToBSON(update)
	if err != nil {
		return nil, err
	}
	mongoOpts := options.FindOneAndUpdate()
	if fo.returnAfter {
		mongoOpts.SetReturnDocument(options.After)
	}
	res := c.inner.FindOneAndUpdate(ctx, filter.raw, u, mongoOpts)
	var raw bson.D
	if err := res.Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoDocuments
		}
		return nil, err
	}
	return bsonToJSON(raw)
}

// FindOneAndDelete atomically finds the first document matching filter, deletes it, and returns it.
// Returns ErrNoDocuments if no document matches.
func (c *mongoCollection) FindOneAndDelete(ctx context.Context, filter Filter) ([]byte, error) {
	res := c.inner.FindOneAndDelete(ctx, filter.raw)
	var raw bson.D
	if err := res.Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoDocuments
		}
		return nil, err
	}
	return bsonToJSON(raw)
}

// DeleteOne deletes the first document matching filter.
func (c *mongoCollection) DeleteOne(ctx context.Context, filter Filter) (DeleteResult, error) {
	res, err := c.inner.DeleteOne(ctx, filter.raw)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// DeleteMany deletes all documents matching filter.
func (c *mongoCollection) DeleteMany(ctx context.Context, filter Filter) (DeleteResult, error) {
	res, err := c.inner.DeleteMany(ctx, filter.raw)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// UpdateSpec specifies a single update operation within a BulkUpdate call.
type UpdateSpec struct {
	Filter Filter
	Update []byte
}

// BulkUpdate applies multiple update operations in a single round-trip.
// Each op updates the first document matching its filter.
func (c *mongoCollection) BulkUpdate(ctx context.Context, ops []UpdateSpec) (UpdateResult, error) {
	models := make([]mongo.WriteModel, len(ops))
	for i, op := range ops {
		u, err := jsonToBSON(op.Update)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("op[%d]: %w", i, err)
		}
		models[i] = mongo.NewUpdateOneModel().SetFilter(op.Filter.raw).SetUpdate(u)
	}
	res, err := c.inner.BulkWrite(ctx, models)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// BulkDelete deletes the first document matching each filter in a single round-trip.
func (c *mongoCollection) BulkDelete(ctx context.Context, filters []Filter) (DeleteResult, error) {
	models := make([]mongo.WriteModel, len(filters))
	for i, f := range filters {
		models[i] = mongo.NewDeleteOneModel().SetFilter(f.raw)
	}
	res, err := c.inner.BulkWrite(ctx, models)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// CountDocuments returns the number of documents matching filter.
func (c *mongoCollection) CountDocuments(ctx context.Context, filter Filter) (int64, error) {
	return c.inner.CountDocuments(ctx, filter.raw)
}

// Aggregate executes a MongoDB aggregation pipeline and returns the result documents as JSON.
// pipeline must be a JSON array of stage documents, e.g.:
//
//	[{"$match":{"status":"active"}},{"$group":{"_id":"$city","count":{"$sum":1}}}]
func (c *mongoCollection) Aggregate(ctx context.Context, pipeline []byte) ([]byte, error) {
	var stages []bson.D
	if err := bson.UnmarshalExtJSON(pipeline, false, &stages); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidJSON, err)
	}

	cur, err := c.inner.Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs [][]byte
	for cur.Next(ctx) {
		var raw bson.D
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := bsonToJSON(raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, data)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return joinJSONArray(docs), nil
}

// IndexOption configures a CreateIndex call.
type IndexOption func(*indexOptions)

type indexOptions struct {
	unique bool
	sparse bool
	ttl    *int32
}

// WithUnique makes the index enforce uniqueness across the collection.
func WithUnique() IndexOption { return func(o *indexOptions) { o.unique = true } }

// WithSparse makes the index skip documents that don't contain the indexed field.
func WithSparse() IndexOption { return func(o *indexOptions) { o.sparse = true } }

// WithTTL creates a TTL index that automatically deletes documents after the given number of seconds.
func WithTTL(seconds int32) IndexOption { return func(o *indexOptions) { o.ttl = &seconds } }

// IndexKey specifies a field and direction for an index.
type IndexKey struct {
	Field     string
	Direction SortDirection
}

// CreateIndex creates an index on one or more fields and returns the index name.
// Pass a single IndexKey for a single-field index, or multiple for a compound index.
func (c *mongoCollection) CreateIndex(ctx context.Context, keys []IndexKey, opts ...IndexOption) (string, error) {
	io := &indexOptions{}
	for _, o := range opts {
		o(io)
	}

	bsonKeys := make(bson.D, len(keys))
	for i, k := range keys {
		dir := 1
		if !k.Direction {
			dir = -1
		}
		bsonKeys[i] = bson.E{Key: k.Field, Value: dir}
	}

	indexOpts := options.Index()
	if io.unique {
		indexOpts.SetUnique(true)
	}
	if io.sparse {
		indexOpts.SetSparse(true)
	}
	if io.ttl != nil {
		indexOpts.SetExpireAfterSeconds(*io.ttl)
	}

	return c.inner.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bsonKeys, Options: indexOpts})
}

// DropIndex drops an index by name.
func (c *mongoCollection) DropIndex(ctx context.Context, name string) error {
	return c.inner.Indexes().DropOne(ctx, name)
}

// ListIndexes returns all indexes on the collection as a JSON array.
func (c *mongoCollection) ListIndexes(ctx context.Context) ([]byte, error) {
	cur, err := c.inner.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs [][]byte
	for cur.Next(ctx) {
		var raw bson.D
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := bsonToJSON(raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, data)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return joinJSONArray(docs), nil
}

// Drop removes the collection from the database.
func (c *mongoCollection) Drop(ctx context.Context) error {
	return c.inner.Drop(ctx)
}

// WithTransaction runs fn inside a single-collection ACID transaction.
// The ctx passed to fn must be forwarded to all collection operations
// so they participate in the transaction. Returning a non-nil error
// from fn aborts the transaction; returning nil commits it.
// Returns ErrReplicaSetRequired if the instance is not a replica set or sharded cluster.
func (c *mongoCollection) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return runWithTransaction(ctx, c.inner.Database().Client(), fn)
}

// Watch opens a change stream on the collection.
// Returns ErrReplicaSetRequired if the instance is not a replica set or sharded cluster.
func (c *mongoCollection) Watch(ctx context.Context, opts ...WatchOption) (ChangeStream, error) {
	wo := &watchOptions{}
	for _, o := range opts {
		o(wo)
	}

	pipeline := bson.A{}
	if len(wo.operationTypes) > 0 {
		types := make(bson.A, len(wo.operationTypes))
		for i, t := range wo.operationTypes {
			types[i] = t
		}
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: types}}}}}})
	}

	streamOpts := options.ChangeStream()
	if wo.fullDocument {
		streamOpts.SetFullDocument(options.UpdateLookup)
	}

	cs, err := c.inner.Watch(ctx, pipeline, streamOpts)
	if err != nil {
		var ce mongo.CommandError
		if errors.As(err, &ce) && ce.Code == 40573 {
			return nil, ErrReplicaSetRequired
		}
		return nil, err
	}
	return &mongoChangeStream{inner: cs}, nil
}

// objectIDToString converts a MongoDB ObjectID (or any _id type) to its hex string.
func objectIDToString(id any) string {
	if id == nil {
		return ""
	}
	if oid, ok := id.(bson.ObjectID); ok {
		return oid.Hex()
	}
	return fmt.Sprintf("%v", id)
}

// joinJSONArray serialises a slice of raw JSON documents into a JSON array.
func joinJSONArray(docs [][]byte) []byte {
	size := 2
	for i, d := range docs {
		size += len(d)
		if i > 0 {
			size++
		}
	}
	buf := make([]byte, 0, size)
	buf = append(buf, '[')
	for i, d := range docs {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, d...)
	}
	return append(buf, ']')
}
