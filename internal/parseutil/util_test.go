package parseutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitStringOnceRune(t *testing.T) {
	var testCases = []struct {
		name  string
		input string
		wantA string
		wantB string
	}{
		{
			name:  "empty string",
			input: "",
			wantA: "",
			wantB: "",
		},
		{
			name:  "no delimiter",
			input: "foo",
			wantA: "foo",
			wantB: "",
		},
		{
			name:  "no latter",
			input: "foo/",
			wantA: "foo",
			wantB: "",
		},
		{
			name:  "no former",
			input: "/foo",
			wantA: "",
			wantB: "foo",
		},
		{
			name:  "only split on first delim",
			input: "foo/bar/moo",
			wantA: "foo",
			wantB: "bar/moo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotA, gotB := splitStringOnceRune(tc.input, '/')
			assert.Equal(t, tc.wantA, gotA)
			assert.Equal(t, tc.wantB, gotB)
		})
	}
}
