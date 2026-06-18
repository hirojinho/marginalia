package agent

import (
	"reflect"
	"strings"
	"testing"
)

func TestCourseName(t *testing.T) {
	if got := CourseName("biology"); !strings.Contains(got, "Biology") {
		t.Fatalf("biology got %q", got)
	}
	if got := CourseName("cs101"); got == "" {
		t.Fatalf("cs101 should be known")
	}
	if got := CourseName("unknown-course-xyz"); got != "" {
		t.Fatalf("unknown should be empty, got %q", got)
	}
}

func TestParsePageSelection(t *testing.T) {
	tests := []struct {
		name  string
		pages string
		total int
		want  []int
	}{
		{"single page", "5", 10, []int{4}},
		{"simple range", "1-3", 10, []int{0, 1, 2}},
		{"mixed list and ranges", "1-3,5,7-8", 10, []int{0, 1, 2, 4, 6, 7}},
		{"whitespace tolerated", " 1 - 3 , 5 ", 10, []int{0, 1, 2, 4}},
		{"deduplicates overlaps", "1,1,2-3,3", 10, []int{0, 1, 2}},
		{"clamps to total on high end", "8-15", 10, []int{7, 8, 9}},
		{"silently drops zero and negatives", "0,-1,2", 10, []int{1}},
		{"empty input yields nil", "", 10, nil},
		{"garbage tokens are skipped", "abc,5", 10, []int{4}},
		{"reversed range yields nothing", "5-3", 10, nil},
		{"total zero yields nothing", "1-3", 0, nil},
		{"page beyond total dropped", "15", 10, nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePageSelection(tc.pages, tc.total)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parsePageSelection(%q, %d) = %v, want %v", tc.pages, tc.total, got, tc.want)
			}
		})
	}
}

func TestPickPages(t *testing.T) {
	pages := []string{"alpha", "bravo", "charlie", "delta"}

	got := pickPages(pages, []int{0, 2})
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if !strings.HasPrefix(got[0], "### Page 1\n") || !strings.Contains(got[0], "alpha") {
		t.Fatalf("first result wrong: %q", got[0])
	}
	if !strings.HasPrefix(got[1], "### Page 3\n") || !strings.Contains(got[1], "charlie") {
		t.Fatalf("second result wrong: %q", got[1])
	}
}

func TestPickPagesIgnoresOutOfRange(t *testing.T) {
	pages := []string{"alpha", "bravo"}
	got := pickPages(pages, []int{-1, 0, 5, 1, 99})
	if len(got) != 2 {
		t.Fatalf("expected 2 in-range results, got %d: %v", len(got), got)
	}
}

func TestPickPagesEmptyIndices(t *testing.T) {
	if got := pickPages([]string{"a"}, nil); got != nil {
		t.Fatalf("expected nil for nil indices, got %v", got)
	}
}
