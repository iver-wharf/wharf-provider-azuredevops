package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			gotAzureOrg, gotAzureProject, gotAzureRepo := parseRepoRefParams(tc.wharfGroup, tc.wharfProject)
			assert.Equal(t, tc.wantAzureOrg, gotAzureOrg)
			assert.Equal(t, tc.wantAzureProject, gotAzureProject)
			assert.Equal(t, tc.wantAzureRepo, gotAzureRepo)
		})
	}
}
