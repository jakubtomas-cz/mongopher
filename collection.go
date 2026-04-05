package mongopher

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection wraps a mongo.Collection and exposes a JSON-native CRUD API.
type Collection struct {
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

// FindOption configures a Find call.
type FindOption func(*findOptions)

type findOptions struct {
	limit int64
	skip  int64
	sort  bson.D
}

// WithLimit limits the number of documents returned.
func WithLimit(n int64) FindOption { return func(o *findOptions) { o.limit = n } }

// WithSkip skips the first n documents.
func WithSkip(n int64) FindOption { return func(o *findOptions) { o.skip = n } }

// WithSort sorts results by the given field. ascending=true for ASC, false for DESC.
func WithSort(field string, ascending bool) FindOption {
	return func(o *findOptions) {
		dir := 1
		if !ascending {
			dir = -1
		}
		o.sort = append(o.sort, bson.E{Key: field, Value: dir})
	}
}

// InsertOne inserts a single JSON document and returns its inserted ID.
func (c *Collection) InsertOne(ctx context.Context, doc []byte) (InsertResult, error) {
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
func (c *Collection) InsertMany(ctx context.Context, docs [][]byte) (InsertManyResult, error) {
	bsons := make([]interface{}, len(docs))
	for i, doc := range docs {
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
func (c *Collection) FindOne(ctx context.Context, filter Filter) ([]byte, error) {
	res := c.inner.FindOne(ctx, filter.raw)
	var raw bson.D
	if err := res.Decode(&raw); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNoDocuments
		}
		return nil, err
	}
	return bsonToJSON(raw)
}

// Find returns all documents matching filter as a slice of JSON.
func (c *Collection) Find(ctx context.Context, filter Filter, opts ...FindOption) ([][]byte, error) {
	fo := &findOptions{}
	for _, o := range opts {
		o(fo)
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

	cur, err := c.inner.Find(ctx, filter.raw, mongoOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results [][]byte
	for cur.Next(ctx) {
		var raw bson.D
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := bsonToJSON(raw)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// UpdateOne updates the first document matching filter using a MongoDB update document (e.g. {"$set":{...}}).
func (c *Collection) UpdateOne(ctx context.Context, filter Filter, update []byte) (UpdateResult, error) {
	u, err := jsonToBSON(update)
	if err != nil {
		return UpdateResult{}, err
	}
	res, err := c.inner.UpdateOne(ctx, filter.raw, u)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// UpdateMany updates all documents matching filter.
func (c *Collection) UpdateMany(ctx context.Context, filter Filter, update []byte) (UpdateResult, error) {
	u, err := jsonToBSON(update)
	if err != nil {
		return UpdateResult{}, err
	}
	res, err := c.inner.UpdateMany(ctx, filter.raw, u)
	if err != nil {
		return UpdateResult{}, err
	}
	return UpdateResult{MatchedCount: res.MatchedCount, ModifiedCount: res.ModifiedCount}, nil
}

// DeleteOne deletes the first document matching filter.
func (c *Collection) DeleteOne(ctx context.Context, filter Filter) (DeleteResult, error) {
	res, err := c.inner.DeleteOne(ctx, filter.raw)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// DeleteMany deletes all documents matching filter.
func (c *Collection) DeleteMany(ctx context.Context, filter Filter) (DeleteResult, error) {
	res, err := c.inner.DeleteMany(ctx, filter.raw)
	if err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// CountDocuments returns the number of documents matching filter.
func (c *Collection) CountDocuments(ctx context.Context, filter Filter) (int64, error) {
	return c.inner.CountDocuments(ctx, filter.raw)
}

// Aggregate executes a MongoDB aggregation pipeline and returns the result documents as JSON.
// pipeline must be a JSON array of stage documents, e.g.:
//
//	[{"$match":{"status":"active"}},{"$group":{"_id":"$city","count":{"$sum":1}}}]
func (c *Collection) Aggregate(ctx context.Context, pipeline []byte) ([][]byte, error) {
	var stages []bson.D
	if err := bson.UnmarshalExtJSON(pipeline, false, &stages); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidJSON, err)
	}

	cur, err := c.inner.Aggregate(ctx, stages)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var results [][]byte
	for cur.Next(ctx) {
		var raw bson.D
		if err := cur.Decode(&raw); err != nil {
			return nil, err
		}
		data, err := bsonToJSON(raw)
		if err != nil {
			return nil, err
		}
		results = append(results, data)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// Drop removes the collection from the database.
func (c *Collection) Drop(ctx context.Context) error {
	return c.inner.Drop(ctx)
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
