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

func NewClient(endpoint string, user, password string) (*Client, error) {
	fmt.Println("Creating Dgraph client...")
	fmt.Println("Endpoint:", endpoint)
	opts := []dgo.ClientOption{
		dgo.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	if user != "" && password != "" {
		opts = append(opts, dgo.WithACLCreds(user, password))
	}

	client, err := dgo.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("could not create Dgraph client: %w", err)
	}

	return &Client{dg: client}, nil
}

func (c *Client) Close() {
	c.dg.Close()
}

func (c *Client) Query(ctx context.Context, query string) (*api.Response, error) {
	txn := c.dg.NewReadOnlyTxn()
	defer txn.Discard(ctx)

	resp, err := txn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying Dgraph: %w", err)
	}
	return resp, nil
}

func (c *Client) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.Mutate(ctx, mutation)
	if err != nil {
		return nil, fmt.Errorf("error mutating Dgraph: %w", err)
	}
	return resp, nil
}

func (c *Client) Upsert(ctx context.Context, query string, mutations []*api.Mutation, commitNow bool) (*api.Response, error) {
	txn := c.dg.NewTxn()
	defer txn.Discard(ctx)

	req := &api.Request{
		Query:     query,
		Mutations: mutations,
		CommitNow: commitNow,
	}
	resp, err := txn.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error performing upsert: %w", err)
	}
	return resp, nil
}
