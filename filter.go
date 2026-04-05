package mongopher

import "go.mongodb.org/mongo-driver/v2/bson"

// Filter holds a parsed MongoDB query predicate.
type Filter struct {
	raw bson.D
}

// FilterFromJSON parses a JSON string into a Filter.
// Returns ErrInvalidJSON if the input is not valid JSON.
func FilterFromJSON(j []byte) (Filter, error) {
	doc, err := jsonToBSON(j)
	if err != nil {
		return Filter{}, err
	}
	return Filter{raw: doc}, nil
}

// EmptyFilter returns a filter that matches all documents.
func EmptyFilter() Filter {
	return Filter{raw: bson.D{}}
}
