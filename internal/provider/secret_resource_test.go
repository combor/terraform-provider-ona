package provider

import (
	"context"
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMapSecretToModel_MapsAllFields(t *testing.T) {
	now := time.Date(2026, time.March, 23, 10, 0, 0, 0, time.UTC)
	// mount is a oneof, so only one of the four mount attributes is ever set.
	secret := &v1.Secret{
		Id:    "secret-123",
		Name:  "DATABASE_URL",
		Mount: &v1.Secret_EnvironmentVariable{EnvironmentVariable: true},
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: "project-456"},
		},
		Creator: &v1.Subject{
			Id:        "user-789",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
		CreatedAt: timestamppb.New(now),
		UpdatedAt: timestamppb.New(now.Add(time.Hour)),
	}

	prior := secretModel{
		Value: types.StringValue("postgres://localhost/db"),
	}

	got := mapSecretToModel(secret, prior)

	assert.Equal(t, "secret-123", got.ID.ValueString())
	assert.Equal(t, "DATABASE_URL", got.Name.ValueString())
	assert.Equal(t, "project-456", got.ProjectID.ValueString())
	assert.True(t, got.EnvironmentVariable.ValueBool())
	assert.True(t, got.FilePath.IsNull())
	assert.True(t, got.ContainerRegistryBasicAuthHost.IsNull())
	assert.False(t, got.APIOnly.ValueBool())
	assert.Equal(t, "user-789", got.CreatorID.ValueString())
	assert.Equal(t, v1.Principal_PRINCIPAL_USER.String(), got.CreatorPrincipal.ValueString())
	assert.Equal(t, now.Format(time.RFC3339Nano), got.CreatedAt.ValueString())
	assert.Equal(t, now.Add(time.Hour).Format(time.RFC3339Nano), got.UpdatedAt.ValueString())
	// Value preserved from prior state
	assert.Equal(t, "postgres://localhost/db", got.Value.ValueString())
}

func TestMapSecretToModel_FilePathMount(t *testing.T) {
	secret := &v1.Secret{
		Id:    "secret-123",
		Name:  "DATABASE_URL",
		Mount: &v1.Secret_FilePath{FilePath: "/etc/secrets/db"},
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: "project-456"},
		},
	}

	got := mapSecretToModel(secret, secretModel{})

	assert.Equal(t, "/etc/secrets/db", got.FilePath.ValueString())
	assert.False(t, got.EnvironmentVariable.ValueBool())
	assert.True(t, got.ContainerRegistryBasicAuthHost.IsNull())
	assert.False(t, got.APIOnly.ValueBool())
}

// environment_variable and api_only are optional-and-computed, so a secret
// created with file_path reads back environment_variable=false. That false must
// not count as a second mount or a replace would fail as a conflict.
func TestApplySecretMount_IgnoresFalseBooleanMounts(t *testing.T) {
	params := &v1.CreateSecretRequest{}
	configured := applySecretMount(params, secretModel{
		EnvironmentVariable: types.BoolValue(false),
		APIOnly:             types.BoolValue(false),
		FilePath:            types.StringValue("/run/secret"),
	})

	assert.Equal(t, []string{"file_path"}, configured)
	assert.Equal(t, "/run/secret", params.GetFilePath())
}

func TestApplySecretMount_IgnoresEmptyStringMounts(t *testing.T) {
	params := &v1.CreateSecretRequest{}
	configured := applySecretMount(params, secretModel{
		EnvironmentVariable:            types.BoolValue(true),
		FilePath:                       types.StringValue(""),
		ContainerRegistryBasicAuthHost: types.StringValue(""),
	})

	assert.Equal(t, []string{"environment_variable"}, configured)
	assert.True(t, params.GetEnvironmentVariable())
}

func TestApplySecretMount_RejectsMoreThanOneMount(t *testing.T) {
	params := &v1.CreateSecretRequest{}
	configured := applySecretMount(params, secretModel{
		EnvironmentVariable: types.BoolValue(true),
		FilePath:            types.StringValue("/etc/secrets/db"),
	})

	assert.Equal(t, []string{"environment_variable", "file_path"}, configured)
}

func TestApplySecretMount_SingleMount(t *testing.T) {
	params := &v1.CreateSecretRequest{}
	configured := applySecretMount(params, secretModel{
		EnvironmentVariable: types.BoolValue(true),
	})

	assert.Equal(t, []string{"environment_variable"}, configured)
	assert.True(t, params.GetEnvironmentVariable())
}

func TestMapSecretToModel_PreservesValueFromPrior(t *testing.T) {
	secret := &v1.Secret{
		Id:   "secret-1",
		Name: "API_KEY",
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: "proj-1"},
		},
	}

	prior := secretModel{
		Value: types.StringValue("super-secret-key"),
	}

	got := mapSecretToModel(secret, prior)
	assert.Equal(t, "super-secret-key", got.Value.ValueString())
}

func TestMapSecretToModel_NullValueWhenPriorIsNull(t *testing.T) {
	secret := &v1.Secret{
		Id:   "secret-1",
		Name: "API_KEY",
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: "proj-1"},
		},
	}

	prior := secretModel{
		Value: types.StringNull(),
	}

	got := mapSecretToModel(secret, prior)
	assert.True(t, got.Value.IsNull())
}

func TestMapSecretToModel_EmptyTimestampsAreNull(t *testing.T) {
	secret := &v1.Secret{
		Id:   "secret-1",
		Name: "API_KEY",
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: "proj-1"},
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
