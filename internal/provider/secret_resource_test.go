package provider

import (
	"testing"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestMapSecretToModel_MapsAllFields(t *testing.T) {
	now := time.Date(2026, time.March, 23, 10, 0, 0, 0, time.UTC)
	secret := gitpod.Secret{
		ID:                             "secret-123",
		Name:                           "DATABASE_URL",
		EnvironmentVariable:            true,
		FilePath:                       "/etc/secrets/db",
		ContainerRegistryBasicAuthHost: "",
		APIOnly:                        false,
		Scope: gitpod.SecretScope{
			ProjectID: "project-456",
		},
		Creator: shared.Subject{
			ID:        "user-789",
			Principal: shared.PrincipalUser,
		},
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	prior := secretModel{
		Value: types.StringValue("postgres://localhost/db"),
	}

	got := mapSecretToModel(secret, prior)

	assert.Equal(t, "secret-123", got.ID.ValueString())
	assert.Equal(t, "DATABASE_URL", got.Name.ValueString())
	assert.Equal(t, "project-456", got.ProjectID.ValueString())
	assert.True(t, got.EnvironmentVariable.ValueBool())
	assert.Equal(t, "/etc/secrets/db", got.FilePath.ValueString())
	assert.True(t, got.ContainerRegistryBasicAuthHost.IsNull())
	assert.False(t, got.APIOnly.ValueBool())
	assert.Equal(t, "user-789", got.CreatorID.ValueString())
	assert.Equal(t, string(shared.PrincipalUser), got.CreatorPrincipal.ValueString())
	assert.Equal(t, now.Format(time.RFC3339Nano), got.CreatedAt.ValueString())
	assert.Equal(t, now.Add(time.Hour).Format(time.RFC3339Nano), got.UpdatedAt.ValueString())
	// Value preserved from prior state
	assert.Equal(t, "postgres://localhost/db", got.Value.ValueString())
}

func TestMapSecretToModel_PreservesValueFromPrior(t *testing.T) {
	secret := gitpod.Secret{
		ID:   "secret-1",
		Name: "API_KEY",
		Scope: gitpod.SecretScope{
			ProjectID: "proj-1",
		},
	}

	prior := secretModel{
		Value: types.StringValue("super-secret-key"),
	}

	got := mapSecretToModel(secret, prior)
	assert.Equal(t, "super-secret-key", got.Value.ValueString())
}

func TestMapSecretToModel_NullValueWhenPriorIsNull(t *testing.T) {
	secret := gitpod.Secret{
		ID:   "secret-1",
		Name: "API_KEY",
		Scope: gitpod.SecretScope{
			ProjectID: "proj-1",
		},
	}

	prior := secretModel{
		Value: types.StringNull(),
	}

	got := mapSecretToModel(secret, prior)
	assert.True(t, got.Value.IsNull())
}

func TestMapSecretToModel_EmptyTimestampsAreNull(t *testing.T) {
	secret := gitpod.Secret{
		ID:   "secret-1",
		Name: "API_KEY",
		Scope: gitpod.SecretScope{
			ProjectID: "proj-1",
		},
	}

	prior := secretModel{
		Value: types.StringValue("val"),
	}

	got := mapSecretToModel(secret, prior)
	assert.True(t, got.CreatedAt.IsNull())
	assert.True(t, got.UpdatedAt.IsNull())
}
