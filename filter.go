package mongopher

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Filter holds a parsed MongoDB query predicate.
type Filter struct {
	raw bson.D
}

// coerceIDValue converts a hex string to a bson.ObjectID when the field is _id.
// This mirrors the flattening applied to documents on the way out, making
// filters round-trip correctly with MongoDB-generated ObjectIDs.
func coerceIDValue(field string, value any) any {
	if field != "_id" {
		return value
	}
	if s, ok := value.(string); ok {
		if oid, err := bson.ObjectIDFromHex(s); err == nil {
			return oid
		}
	}
	return value
}

// FilterFromJSON parses a JSON string into a Filter.
// Returns ErrInvalidJSON if the input is not valid JSON.
func FilterFromJSON(j []byte) (Filter, error) {
	doc, err := jsonToBSON(j)
	if err != nil {
		return Filter{}, err
	}
	for i, elem := range doc {
		if elem.Key == "_id" {
			doc[i].Value = coerceIDValue("_id", elem.Value)
			break
		}
	}
	return Filter{raw: doc}, nil
}

// EmptyFilter returns a filter that matches all documents.
func EmptyFilter() Filter {
	return Filter{raw: bson.D{}}
}

// Eq matches documents where field equals value.
func Eq(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: coerceIDValue(field, value)}}}
}

// Ne matches documents where field does not equal value.
func Ne(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$ne", Value: coerceIDValue(field, value)}}}}}
}

// Gt matches documents where field is greater than value.
func Gt(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$gt", Value: coerceIDValue(field, value)}}}}}
}

// Gte matches documents where field is greater than or equal to value.
func Gte(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$gte", Value: coerceIDValue(field, value)}}}}}
}

// Lt matches documents where field is less than value.
func Lt(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$lt", Value: coerceIDValue(field, value)}}}}}
}

// Lte matches documents where field is less than or equal to value.
func Lte(field string, value any) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$lte", Value: coerceIDValue(field, value)}}}}}
}

// Exists matches documents where field is present (exists true) or absent (exists false).
func Exists(field string, exists bool) Filter {
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$exists", Value: exists}}}}}
}

// In matches documents where field equals any of the provided values.
func In(field string, values ...any) Filter {
	coerced := make([]any, len(values))
	for i, v := range values {
		coerced[i] = coerceIDValue(field, v)
	}
	return Filter{raw: bson.D{{Key: field, Value: bson.D{{Key: "$in", Value: coerced}}}}}
}

// And combines multiple filters into a single filter that matches documents
// satisfying all of them.
func And(filters ...Filter) Filter {
	conditions := make(bson.A, len(filters))
	for i, f := range filters {
		conditions[i] = f.raw
	}
	return Filter{raw: bson.D{{Key: "$and", Value: conditions}}}
}

// Or combines multiple filters into a single filter that matches documents
// satisfying at least one of them.
func Or(filters ...Filter) Filter {
	conditions := make(bson.A, len(filters))
	for i, f := range filters {
		conditions[i] = f.raw
	}
	return Filter{raw: bson.D{{Key: "$or", Value: conditions}}}
}

// FilterByID returns a filter that matches a document by its _id field.
func FilterByID(id string) (Filter, error) {
	v, err := json.Marshal(id)
	if err != nil {
		return Filter{}, ErrInvalidJSON
	}
	j := append([]byte(`{"_id":`), append(v, '}')...)
	return FilterFromJSON(j)
}
