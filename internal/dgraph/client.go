package dgraph

import (
	"context"
	"fmt"

	"github.com/dgraph-io/dgo/v240"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"google.golang.org/grpc"
)

type Client struct {
	dg *dgo.Dgraph
}

func NewClient(endpoint string) (*Client, error) {
	conn, err := grpc.Dial(endpoint, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to Dgraph: %w", err)
	}

	return &Client{
		dg: dgo.NewDgraphClient(api.NewDgraphClient(conn)),
	}, nil
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
