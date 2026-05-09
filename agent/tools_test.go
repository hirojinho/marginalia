package agent

import (
	"reflect"
	"testing"
)

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
