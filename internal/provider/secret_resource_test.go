package provider

import (
	"context"
	"testing"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestParseSecretImportID(t *testing.T) {
	t.Run("valid composite id", func(t *testing.T) {
		projectID, secretID, err := parseSecretImportID("project-123/secret-456")

		require.NoError(t, err)
		assert.Equal(t, "project-123", projectID)
		assert.Equal(t, "secret-456", secretID)
	})

	t.Run("rejects missing separator", func(t *testing.T) {
		_, _, err := parseSecretImportID("secret-456")

		require.EqualError(t, err, "expected import identifier in the format <project-id>/<secret-id>")
	})

	t.Run("rejects empty components", func(t *testing.T) {
		testCases := []string{
			"/secret-456",
			"project-123/",
		}

		for _, testCase := range testCases {
			_, _, err := parseSecretImportID(testCase)
			require.EqualError(t, err, "expected import identifier in the format <project-id>/<secret-id>")
		}
	})
}

func TestSecretResourceImportState(t *testing.T) {
	ctx := context.Background()
	r := &secretResource{}
	schema := secretTestSchema(t)

	resp := resource.ImportStateResponse{
		State: tfsdk.State{
			Schema: schema,
			Raw: tftypes.NewValue(
				schema.Type().TerraformType(ctx),
				nil,
			),
		},
	}

	r.ImportState(ctx, resource.ImportStateRequest{ID: "project-123/secret-456"}, &resp)

	require.False(t, resp.Diagnostics.HasError(), "unexpected diagnostics: %v", resp.Diagnostics)

	var projectID types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("project_id"), &projectID)...)
	var secretID types.String
	resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &secretID)...)

	require.False(t, resp.Diagnostics.HasError(), "unexpected diagnostics: %v", resp.Diagnostics)
	assert.Equal(t, "project-123", projectID.ValueString())
	assert.Equal(t, "secret-456", secretID.ValueString())
}

func TestSecretResourceImportState_InvalidID(t *testing.T) {
	ctx := context.Background()
	r := &secretResource{}
	schema := secretTestSchema(t)

	resp := resource.ImportStateResponse{
		State: tfsdk.State{
			Schema: schema,
			Raw: tftypes.NewValue(
				schema.Type().TerraformType(ctx),
				nil,
			),
		},
	}

	r.ImportState(ctx, resource.ImportStateRequest{ID: "secret-456"}, &resp)

	require.True(t, resp.Diagnostics.HasError())
	assert.Contains(t, resp.Diagnostics[0].Summary(), "Invalid import ID")
	assert.Contains(t, resp.Diagnostics[0].Detail(), "<project-id>/<secret-id>")
}

func secretTestSchema(t *testing.T) resourceschema.Schema {
	t.Helper()

	r := &secretResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	return resp.Schema
}
