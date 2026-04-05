package mongopher

import "errors"

// ErrNoDocuments is returned by FindOne when no document matches the filter.
var ErrNoDocuments = errors.New("mongopher: no documents found")

// ErrInvalidJSON is returned when a JSON argument cannot be parsed.
var ErrInvalidJSON = errors.New("mongopher: invalid JSON")

// ErrTransactionsNotSupported is returned by WithTransaction when the MongoDB
// instance does not support multi-document transactions (i.e. it is not a
// replica set member or mongos).
var ErrTransactionsNotSupported = errors.New("mongopher: transactions require a replica set or sharded cluster")
