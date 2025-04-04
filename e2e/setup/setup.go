package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/dgraph-io/dgo/v240"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupSchema(dg *dgo.Dgraph) {
	ctx := context.Background()
	op := &api.Operation{
		Schema: `
            name: string @index(exact) .
            email: string @index(exact) .
            type Person {
                name
                email
            }
        `,
	}
	err := dg.Alter(ctx, op)
	if err != nil {
		log.Fatalf("Error setting up schema: %v", err)
	}
}

func insertSampleData(dg *dgo.Dgraph) {
	ctx := context.Background()
	p := struct {
		Uid   string   `json:"uid,omitempty"`
		Name  string   `json:"name,omitempty"`
		Email string   `json:"email,omitempty"`
		DType []string `json:"dgraph.type,omitempty"`
	}{
		Uid:   "_:alice",
		Name:  "Alice",
		Email: "alice@example.com",
		DType: []string{"Person"},
	}

	mu := &api.Mutation{
		CommitNow: true,
	}
	pb, err := json.Marshal(p)
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}
	mu.SetJson = pb

	_, err = dg.NewTxn().Mutate(ctx, mu)
	if err != nil {
		log.Fatalf("Error inserting sample data: %v", err)
	}
}

func setup(dg *dgo.Dgraph) {
	setupSchema(dg)
	insertSampleData(dg)
}

func main() {

	opts := []dgo.ClientOption{
		dgo.WithGrpcOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	dg, err := dgo.NewClient("localhost:9080", opts...)
	if err != nil {
		fmt.Println("could not create Dgraph client: %w", err)
	}
	setup(dg)
}
