package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func newCORSReq(origin string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

func TestApplyCORS_NonDevEmptyAllowListWritesNothing(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://attacker.example"), nil, false)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("non-dev empty allow-list should not echo Origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("non-dev empty allow-list must not enable credentials, got %q", got)
	}
}

func TestApplyCORS_AllowListReflectsOnlyMatching(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://app.example.com"),
		[]string{"https://app.example.com"}, false)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("expected exact-match origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("credentials should be enabled for matched origin, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary: Origin missing, got %q", got)
	}
}

func TestApplyCORS_AllowListIgnoresUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://attacker.example"),
		[]string{"https://app.example.com"}, false)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unmatched origin should not be reflected, got %q", got)
	}
}

func TestApplyCORS_WildcardSubdomainMatch(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://api.example.com"),
		[]string{"*.example.com"}, false)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://api.example.com" {
		t.Fatalf("expected wildcard subdomain to match, got %q", got)
	}
}

func TestApplyCORS_StarPatternOmitsCredentials(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://something"), []string{"*"}, false)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected `*`, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("`*` allow-origin must NOT carry credentials, got %q", got)
	}
}

func TestApplyCORS_DevModeEmptyEchoesOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	applyCORS(rec, newCORSReq("https://anywhere"), nil, true)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://anywhere" {
		t.Fatalf("dev-mode empty list should echo Origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("dev-mode legacy behaviour expected credentials=true, got %q", got)
	}
}

// BenchmarkIsDQL_LongQuery exercises the line-walker against a multi-
// line query to confirm the strings.Split allocation is gone. With the
// previous implementation this benchmark reported ~lines+1 allocations
// per call; the streaming version should report 0 allocs/op.
func BenchmarkIsDQL_LongQuery(b *testing.B) {
	var sb []byte
	for i := 0; i < 200; i++ {
		sb = append(sb, []byte("# comment line that is reasonably long\n")...)
	}
	sb = append(sb, []byte("{ q(func: has(name)) { uid name } }\n")...)
	src := string(sb)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !isDQL(src) {
			b.Fatal("expected DQL")
		}
	}
}
