package mongopher

import "errors"

// ErrNoDocuments is returned by FindOne when no document matches the filter.
var ErrNoDocuments = errors.New("mongopher: no documents found")

// ErrInvalidJSON is returned when a JSON argument cannot be parsed.
var ErrInvalidJSON = errors.New("mongopher: invalid JSON")
