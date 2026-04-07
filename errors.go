package mongopher

import "errors"

// ErrNoDocuments is returned by FindOne when no document matches the filter.
var ErrNoDocuments = errors.New("mongopher: no documents found")

// ErrInvalidJSON is returned when a JSON argument cannot be parsed.
var ErrInvalidJSON = errors.New("mongopher: invalid JSON")

// ErrReplicaSetRequired is returned by WithTransaction and Watch when the
// MongoDB instance is a standalone node rather than a replica set or sharded cluster.
var ErrReplicaSetRequired = errors.New("mongopher: requires a replica set or sharded cluster")
