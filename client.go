package mongopher

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Client wraps the MongoDB client and holds the target database name.
type Client struct {
	inner  *mongo.Client
	dbName string
}

// Connect creates a new Client connected to the given URI and database.
// Additional driver options (e.g. TLS, auth) can be passed as extra arguments
// and are merged after the base URI options.
func Connect(ctx context.Context, uri, dbName string, opts ...*options.ClientOptions) (*Client, error) {
	base := options.Client().ApplyURI(uri)
	inner, err := mongo.Connect(append([]*options.ClientOptions{base}, opts...)...)
	if err != nil {
		return nil, err
	}
	if err := inner.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &Client{inner: inner, dbName: dbName}, nil
}

// WithTransaction runs fn inside a multi-document ACID transaction.
// The ctx passed to fn must be forwarded to all collection operations
// so they participate in the transaction. Returning a non-nil error
// from fn aborts the transaction; returning nil commits it.
// Returns ErrReplicaSetRequired if the instance is not a replica set or sharded cluster.
func (c *Client) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return runWithTransaction(ctx, c.inner, fn)
}

func runWithTransaction(ctx context.Context, client *mongo.Client, fn func(ctx context.Context) error) error {
	session, err := client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(mongo.NewSessionContext(ctx, session), func(ctx context.Context) (any, error) {
		return nil, fn(ctx)
	})
	if err != nil {
		var ce mongo.CommandError
		if errors.As(err, &ce) && ce.Code == 20 {
			return ErrReplicaSetRequired
		}
		return err
	}
	return nil
}

// Driver returns the underlying *mongo.Client for operations not covered by mongopher.
func (c *Client) Driver() *mongo.Client {
	return c.inner
}

// Disconnect closes the underlying connection.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.inner.Disconnect(ctx)
}

// Collection returns a handle for the named collection.
func (c *Client) Collection(name string) Collection {
	return &mongoCollection{
		inner: c.inner.Database(c.dbName).Collection(name),
	}
}
