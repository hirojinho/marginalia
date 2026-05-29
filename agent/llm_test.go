package agent

import "testing"

func TestCleanTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Explain Raft consensus", "Explain Raft consensus"},
		{"strips wrapping double quotes", "\"Raft Consensus Basics\"", "Raft Consensus Basics"},
		{"strips wrapping single quotes", "'Replication Lag'", "Replication Lag"},
		{"trims whitespace", "  Title with spaces  ", "Title with spaces"},
		{"strips trailing period", "Quorum reads and writes.", "Quorum reads and writes"},
		{"collapses inner whitespace/newlines", "Two\n\nWord", "Two Word"},
		{"empty stays empty", "   ", ""},
		{"truncates over 60 runes", "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggg", "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee fffff…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanTitle(c.in); got != c.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
