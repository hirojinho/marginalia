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

func TestParseGradeResponse(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		grade   int
		just    string
		wantErr  bool
	}{
		{"valid", `{"grade":4,"justification":"Mostly correct"}`, 4, "Mostly correct", false},
		{"grade 0", `{"grade":0,"justification":"No answer"}`, 0, "No answer", false},
		{"grade 5", `{"grade":5,"justification":"Perfect"}`, 5, "Perfect", false},
		{"grade out of range", `{"grade":6,"justification":"Too high"}`, 0, "", true},
		{"negative grade", `{"grade":-1,"justification":"Negative"}`, 0, "", true},
		{"invalid JSON", `not json`, 0, "", true},
		{"missing grade key defaults to 0", `{"justification":"no grade"}`, 0, "no grade", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			grade, just, err := parseGradeResponse(c.raw)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr {
				if grade != c.grade {
					t.Errorf("grade = %d, want %d", grade, c.grade)
				}
				if just != c.just {
					t.Errorf("justification = %q, want %q", just, c.just)
				}
			}
		})
	}
}
