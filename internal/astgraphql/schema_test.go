package astgraphql

import (
	"encoding/json"
	"testing"

	"github.com/OpenDgraph/Otter/internal/helpers"
)

// func TestParseSchema(t *testing.T) {
// 	sdl := `
// 		type User {
// 			id: ID!
// 			name: String!
// 			friend: User
// 		}

// 		type Query {
// 			user(id: ID!): User
// 		}
// 	`

// 	schema, err := ParseSchema(sdl)
// 	if err != nil {
// 		t.Fatalf("Failed to parse schema: %v", err)
// 	}

// 	if schema.Types["User"] == nil {
// 		t.Errorf("Expected type 'User' not found")
// 	}

// 	if schema.Types["Query"] == nil {
// 		t.Errorf("Expected type 'Query' not found")
// 	}
// }

func TestSchemaToJSON(t *testing.T) {
	sdl := `
		type TodoList {
			id: ID!
			title: String! @search(by: [term])
			todos: [TodoItem] @hasInverse(field: list)

			createdAt: DateTime
			updatedAt: DateTime
		}

		type TodoItem {
			id: ID!
			text: String! @search(by: [term])
			done: Boolean! 
			dueDate: DateTime
			list: TodoList!

			createdAt: DateTime
			updatedAt: DateTime
		}

		enum Role {
			ADMIN
			USER
			GUEST
		}

		interface Entity {
			id: ID!
		}

		type User implements Entity {
			id: ID!
			name: String!
			role: Role
			friend: User
		}

		type Admin implements Entity {
			id: ID!
			powers: [String!]!
		}

		union Account = User | Admin

		input NewUserInput {
			name: String!
			role: Role!
		}

	`

	schema, err := ParseSchema(sdl)
	if err != nil {
		t.Fatalf("Failed to parse schema: %v", err)
	}

	jsonData, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("Failed to convert schema to JSON: %v", err)
	}

	var parsedAST any
	if err := json.Unmarshal(jsonData, &parsedAST); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	snapshot := helpers.ASTSnapshot{
		Test:  "TestSchemaToJSON",
		Input: sdl,
		AST:   parsedAST,
	}

	helpers.SaveASTSnapshot(t, snapshot, "testdata/snapshots/schema_ast.json")
}
