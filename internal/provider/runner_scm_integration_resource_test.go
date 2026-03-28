package provider

import (
	"testing"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestMapScmIntegrationToModel_MapsAllFields(t *testing.T) {
	integration := gitpod.ScmIntegration{
		ID:       "int-123",
		RunnerID: "runner-456",
		ScmID:    "github",
		Host:     "github.com",
		OAuth: gitpod.ScmIntegrationOAuthConfig{
			ClientID:  "oauth-client-id",
			IssuerURL: "https://auth.example.com",
		},
		Pat:              true,
		VirtualDirectory: "/tfs",
	}

	prior := runnerScmIntegrationModel{
		OAuthPlaintextClientSecret: types.StringValue("super-secret"),
	}

	got := mapScmIntegrationToModel(integration, prior)

	assert.Equal(t, "int-123", got.ID.ValueString())
	assert.Equal(t, "runner-456", got.RunnerID.ValueString())
	assert.Equal(t, "github", got.ScmID.ValueString())
	assert.Equal(t, "github.com", got.Host.ValueString())
	assert.Equal(t, "oauth-client-id", got.OAuthClientID.ValueString())
	assert.True(t, got.Pat.ValueBool())
	assert.Equal(t, "https://auth.example.com", got.IssuerURL.ValueString())
	assert.Equal(t, "/tfs", got.VirtualDirectory.ValueString())
	assert.Equal(t, "super-secret", got.OAuthPlaintextClientSecret.ValueString())
}

func TestMapScmIntegrationToModel_PatOnly(t *testing.T) {
	integration := gitpod.ScmIntegration{
		ID:       "int-789",
		RunnerID: "runner-abc",
		ScmID:    "github",
		Host:     "github.com",
		Pat:      true,
	}

	prior := runnerScmIntegrationModel{
		OAuthPlaintextClientSecret: types.StringNull(),
	}

	got := mapScmIntegrationToModel(integration, prior)

	assert.Equal(t, "int-789", got.ID.ValueString())
	assert.Equal(t, "runner-abc", got.RunnerID.ValueString())
	assert.True(t, got.Pat.ValueBool())
	assert.True(t, got.OAuthClientID.IsNull())
	assert.True(t, got.IssuerURL.IsNull())
	assert.True(t, got.VirtualDirectory.IsNull())
	assert.True(t, got.OAuthPlaintextClientSecret.IsNull())
}

func TestMapScmIntegrationToModel_PreservesClientSecret(t *testing.T) {
	integration := gitpod.ScmIntegration{
		ID:       "int-1",
		RunnerID: "runner-1",
		ScmID:    "gitlab",
		Host:     "gitlab.com",
		OAuth: gitpod.ScmIntegrationOAuthConfig{
			ClientID: "client-id",
		},
	}

	prior := runnerScmIntegrationModel{
		OAuthPlaintextClientSecret: types.StringValue("my-secret-value"),
	}

	got := mapScmIntegrationToModel(integration, prior)
	assert.Equal(t, "my-secret-value", got.OAuthPlaintextClientSecret.ValueString())
}
