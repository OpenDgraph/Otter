package websocket

import "testing"

func TestMatchOrigin_Wildcard(t *testing.T) {
	if !MatchOrigin("http://anything.example", []string{"*"}) {
		t.Fatal("'*' must match any origin")
	}
}

func TestMatchOrigin_ExactScheme(t *testing.T) {
	patterns := []string{"https://app.example.com"}
	if !MatchOrigin("https://app.example.com", patterns) {
		t.Fatal("exact URL must match")
	}
	if MatchOrigin("http://app.example.com", patterns) {
		t.Fatal("scheme mismatch must not match")
	}
}

func TestMatchOrigin_Subdomain(t *testing.T) {
	patterns := []string{"*.example.com"}
	cases := map[string]bool{
		"https://api.example.com":          true,
		"https://deep.nested.example.com":  true,
		"https://example.com":              true,
		"https://api.example.co":           false,
		"https://evil.example.com.attack":  false,
		"https://attackerexample.com":      false,
	}
	for origin, want := range cases {
		if got := MatchOrigin(origin, patterns); got != want {
			t.Errorf("MatchOrigin(%q) = %v, want %v", origin, got, want)
		}
	}
}

func TestMatchOrigin_EmptyListFailsClosed(t *testing.T) {
	if MatchOrigin("https://app.example.com", nil) {
		t.Fatal("empty allow-list must reject")
	}
	if MatchOrigin("https://app.example.com", []string{}) {
		t.Fatal("empty allow-list must reject")
	}
}

func TestMatchOrigin_MalformedOrigin(t *testing.T) {
	if MatchOrigin("not-a-url", []string{"https://app.example.com"}) {
		t.Fatal("unparseable origin must not match explicit allow-list")
	}
}

func TestConstantTimeTokenEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "", false},
		{"abc", "", false},
		{"", "abc", false},
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "abcd", false},
	}
	for _, tc := range cases {
		if got := ConstantTimeTokenEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("ConstantTimeTokenEqual(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
