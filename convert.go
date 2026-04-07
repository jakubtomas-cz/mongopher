package mongopher

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// jsonToBSON parses relaxed Extended JSON into a bson.D.
func jsonToBSON(data []byte) (bson.D, error) {
	var doc bson.D
	if err := bson.UnmarshalExtJSON(data, false, &doc); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidJSON, err)
	}
	return doc, nil
}

// bsonToJSON serialises a bson.D to relaxed Extended JSON and then flattens
// the top-level _id field from {"$oid":"..."} to a plain string.
func bsonToJSON(doc bson.D) ([]byte, error) {
	data, err := bson.MarshalExtJSON(doc, false, false)
	if err != nil {
		return nil, err
	}
	return flattenID(data), nil
}

// Marshal is a thin wrapper over json.Marshal that encodes a Go value to JSON bytes,
// ready to pass to InsertOne, Set, or similar methods.
//
//	doc, err := mongopher.Marshal(map[string]any{"title": "Buy milk", "done": false})
//	col.InsertOne(ctx, doc)
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalAs is a thin wrapper over json.Unmarshal that decodes JSON into T.
// T determines the shape — use a slice type for arrays and a non-slice type for single documents.
//
//	// JSON array from Find/Aggregate
//	users, err := mongopher.UnmarshalAs[[]User](docs)
//
//	// Single document from FindOne
//	user, err := mongopher.UnmarshalAs[User](doc)
func UnmarshalAs[T any](docs []byte) (T, error) {
	var result T
	if err := json.Unmarshal(docs, &result); err != nil {
		var zero T
		return zero, fmt.Errorf("%w: %s", ErrInvalidJSON, err)
	}
	return result, nil
}

// flattenID replaces a top-level "_id":{"$oid":"<hex>"} with "_id":"<hex>".
// If _id is not an Extended JSON object or is absent the data is returned unchanged.
func flattenID(data []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return data
	}
	idRaw, ok := raw["_id"]
	if !ok {
		return data
	}

	// Try to decode as {"$oid": "<hex>"}
	var oidWrapper struct {
		OID string `json:"$oid"`
	}
	if err := json.Unmarshal(idRaw, &oidWrapper); err == nil && oidWrapper.OID != "" {
		plain, err := json.Marshal(oidWrapper.OID)
		if err != nil {
			return data
		}
		raw["_id"] = plain
		out, err := json.Marshal(raw)
		if err != nil {
			return data
		}
		return out
	}

	return data
}
