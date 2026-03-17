package events

import (
	"reflect"
	"testing"
)

func TestUniqueTopics(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty input returns empty",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single topic preserved",
			input: []string{"user.created"},
			want:  []string{"user.created"},
		},
		{
			name:  "duplicates removed",
			input: []string{"user.created", "user.created", "user.updated"},
			want:  []string{"user.created", "user.updated"},
		},
		{
			name:  "order preserved (first occurrence wins)",
			input: []string{"user.deleted", "user.created", "user.updated", "user.created", "user.deleted"},
			want:  []string{"user.deleted", "user.created", "user.updated"},
		},
		{
			name:  "all unique returns same slice content",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "all duplicates collapses to one",
			input: []string{"x", "x", "x"},
			want:  []string{"x"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueTopics(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("uniqueTopics(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
