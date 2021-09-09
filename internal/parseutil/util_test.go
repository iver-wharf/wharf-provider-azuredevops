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

func TestParseRepoRefParams(t *testing.T) {
	var testCases = []struct {
		name             string
		wharfGroup       string
		wharfProject     string
		wantAzureOrg     string
		wantAzureProject string
		wantAzureRepo    string
	}{
		{
			name:             "old v1 format",
			wharfGroup:       "Org",
			wharfProject:     "Proj",
			wantAzureOrg:     "Org",
			wantAzureProject: "Proj",
			wantAzureRepo:    "",
		},
		{
			name:             "new v2 format",
			wharfGroup:       "Org/Proj",
			wharfProject:     "Repo",
			wantAzureOrg:     "Org",
			wantAzureProject: "Proj",
			wantAzureRepo:    "Repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotAzureOrg, gotAzureProject, gotAzureRepo := ParseRepoRefParams(tc.wharfGroup, tc.wharfProject)
			assert.Equal(t, tc.wantAzureOrg, gotAzureOrg)
			assert.Equal(t, tc.wantAzureProject, gotAzureProject)
			assert.Equal(t, tc.wantAzureRepo, gotAzureRepo)
		})
	}
}
