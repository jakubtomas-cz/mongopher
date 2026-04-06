package mongopher

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Filter holds a parsed MongoDB query predicate.
type Filter struct {
	raw bson.D
}

// FilterFromJSON parses a JSON string into a Filter.
// Returns ErrInvalidJSON if the input is not valid JSON.
// A string _id that is a valid ObjectID hex is automatically coerced to a
// BSON ObjectID, mirroring the flattening applied to documents on the way out.
func FilterFromJSON(j []byte) (Filter, error) {
	doc, err := jsonToBSON(j)
	if err != nil {
		return Filter{}, err
	}
	for i, elem := range doc {
		if elem.Key == "_id" {
			if s, ok := elem.Value.(string); ok {
				if oid, err := bson.ObjectIDFromHex(s); err == nil {
					doc[i].Value = oid
				}
			}
			break
		}
	}
	return Filter{raw: doc}, nil
}

// EmptyFilter returns a filter that matches all documents.
func EmptyFilter() Filter {
	return Filter{raw: bson.D{}}
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
