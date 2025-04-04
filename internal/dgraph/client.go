package dgraph

import (
	"context"
	"fmt"

	"github.com/dgraph-io/dgo/v240"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	dg *dgo.Dgraph
}

func NewClient(endpoints string, user, password string) (*Client, error) {
	opts := []dgo.ClientOption{
		dgo.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		dgo.WithGrpcOption(grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"retryPolicy": {
					"MaxAttempts": 4
				}]
		}`)),
	}

	if user != "" && password != "" {
		opts = append(opts, dgo.WithACLCreds(user, password))
	}

	client, err := dgo.NewClient(endpoints, opts...)
	if err != nil {
		return nil, fmt.Errorf("could not create Dgraph client: %w", err)
	}

	return &Client{dg: client}, nil
}

func (c *Client) Close() {
	c.dg.Close()
}

func (c *Client) Query(ctx context.Context, query string) (*api.Response, error) {
	resp, err := c.dg.NewReadOnlyTxn().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying Dgraph: %w", err)
	}
	return resp, nil
}

func (c *Client) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
	resp, err := c.dg.NewTxn().Mutate(ctx, mutation)
	if err != nil {
		return nil, fmt.Errorf("error mutating Dgraph: %w", err)
	}
	return resp, nil
}
