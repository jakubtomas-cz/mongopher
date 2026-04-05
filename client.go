package mongopher

import (
	"context"

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

// Disconnect closes the underlying connection.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.inner.Disconnect(ctx)
}

// Collection returns a handle for the named collection.
func (c *Client) Collection(name string) *Collection {
	return &Collection{
		inner: c.inner.Database(c.dbName).Collection(name),
	}
}
