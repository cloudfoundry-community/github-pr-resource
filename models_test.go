package resource_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	resource "github.com/telia-oss/github-pr-resource"
)

func TestSource(t *testing.T) {
	tests := []struct {
		description string
		source      resource.Source
		wantErr     string
	}{
		{
			description: "validate passes",
			source: resource.Source{
				AccessToken: "123456",
				Repository:  "test/test",
			},
		},
		{
			description: "should have an access_token",
			source: resource.Source{
				Repository: "test/test",
			},
			wantErr: "access_token must be set if not using GitHub App authentication",
		},
		{
			description: "should have a repository",
			source: resource.Source{
				AccessToken: "123456",
			},
			wantErr: "repository must be set",
		},
		{
			description: "should support GitHub App authentication",
			source: resource.Source{
				Repository:     "test/test",
				UseGitHubApp:   true,
				PrivateKey:     "key.pem",
				ApplicationID:  123456,
				InstallationID: 1,
			},
		},
		{
			description: "requires a private_key or private_key_file GitHub App configuration values",
			source: resource.Source{
				Repository:     "test/test",
				UseGitHubApp:   true,
				ApplicationID:  123456,
				InstallationID: 1,
			},
			wantErr: "Either private_key or private_key_file should be supplied if using GitHub App authentication",
		},
		{
			description: "requires an application_id and installation_id GitHub App configuration values",
			source: resource.Source{
				Repository:    "test/test",
				UseGitHubApp:  true,
				PrivateKey:    "key.pem",
				ApplicationID: 123456,
			},
			wantErr: "application_id and installation_id must be set if using GitHub App authentication",
		},
		{
			description: "should not have an access_token when using GitHub App authentication",
			source: resource.Source{
				Repository:     "test/test",
				UseGitHubApp:   true,
				PrivateKey:     "key.pem",
				ApplicationID:  123456,
				InstallationID: 1,
				AccessToken:    "123456",
			},
			wantErr: "access_token is not required when using GitHub App authentication",
		},
		{
			description: "requires v3_endpoint when v4_endpoint is set",
			source: resource.Source{
				AccessToken: "123456",
				Repository:  "test/test",
				V3Endpoint:  "https://github.com/v3",
			},
			wantErr: "v4_endpoint must be set together with v3_endpoint",
		},
		{
			description: "requires v4_endpoint when v3_endpoint is set",
			source: resource.Source{
				AccessToken: "123456",
				Repository:  "test/test",
				V4Endpoint:  "https://github.com/v4",
			},
			wantErr: "v3_endpoint must be set together with v4_endpoint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			err := tc.source.Validate()

			if tc.wantErr != "" {
				if err == nil {
					t.Logf("Expected error '%s', got nothing", tc.wantErr)
					t.Fail()
				}
				assert.EqualError(t, err, tc.wantErr, fmt.Sprintf("Expected '%s', got '%s'", tc.wantErr, err))
			}

			if tc.wantErr == "" && err != nil {
				t.Logf("Got an error when none expected: %s", err)
				t.Fail()
			}
		})
	}
}
