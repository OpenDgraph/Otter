package proxy

import "testing"

func TestIsDQL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"dql_func", "{ q(func: has(name)) { uid } }", true},
		{"schema_literal", "schema {}", true},
		{
			name: "comment_only_schema_ignored",
			in:   "# schema {}\nquery { me { name } }",
			want: false,
		},
		{
			name: "comment_only_func_ignored",
			in:   "# func:has(name)\nquery { me { name } }",
			want: false,
		},
		{"plain_graphql", "query { me { name } }", false},
		{"empty", "", false},
		{"whitespace", "   \n\t  \n", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDQL(tc.in); got != tc.want {
				t.Fatalf("isDQL(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
