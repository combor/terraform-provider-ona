package provider

import (
	"context"
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMapProjectToDataSourceModel_MapsComputedFields(t *testing.T) {
	project := &v1.Project{}
	raw := `{
		"prebuildConfiguration": {
			"enabled": true,
			"enableJetbrainsWarmup": true,
			"environmentClassIds": ["env-1"],
			"timeout": "3600s",
			"executor": {
				"id": "subject-1",
				"principal": "PRINCIPAL_USER"
			}
		}
	}`
	require.NoError(t, protojson.Unmarshal([]byte(raw), project))

	project.Id = "project-456"
	project.AutomationsFilePath = ".gitpod/automations.yaml"
	project.DevcontainerFilePath = ".devcontainer/devcontainer.json"
	project.DesiredPhase = v1.ProjectPhase_PROJECT_PHASE_ACTIVE
	// spec is a oneof, so context_url and git each need their own entry.
	project.Initializer = &v1.EnvironmentInitializer{
		Specs: []*v1.EnvironmentInitializer_Spec{
			{
				Spec: &v1.EnvironmentInitializer_Spec_ContextUrl{
					ContextUrl: &v1.ContextURLInitializer{
						Url: "https://example.com/context",
					},
				},
			},
			{
				Spec: &v1.EnvironmentInitializer_Spec_Git{
					Git: &v1.GitInitializer{
						RemoteUri: "https://github.com/combor/terraform-provider-ona",
					},
				},
			},
		},
	}
	project.RecommendedEditors = &v1.RecommendedEditors{
		Editors: map[string]*v1.EditorVersions{
			"vscode": {Versions: []string{"stable"}},
		},
	}
	project.TechnicalDescription = "project description"
	project.Metadata = &v1.ProjectMetadata{
		Name:           "project-name",
		OrganizationId: "org-123",
		CreatedAt:      timestamppb.New(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)),
		UpdatedAt:      timestamppb.New(time.Date(2026, time.January, 3, 4, 5, 6, 0, time.UTC)),
		Creator: &v1.Subject{
			Id:        "creator-1",
			Principal: v1.Principal_PRINCIPAL_USER,
		},
	}
	project.UsedBy = &v1.Project_UsedBy{
		TotalSubjects: 1,
		Subjects: []*v1.Subject{{
			Id:        "user-1",
			Principal: v1.Principal_PRINCIPAL_USER,
		}},
	}

	got, diags := mapProjectToDataSourceModel(context.Background(), project)
	require.False(t, diags.HasError())

	assert.Equal(t, "project-456", got.ID.ValueString())
	assert.Equal(t, "project-name", got.Name.ValueString())
	assert.Equal(t, ".gitpod/automations.yaml", got.AutomationsFilePath.ValueString())
	assert.Equal(t, ".devcontainer/devcontainer.json", got.DevcontainerFilePath.ValueString())
	assert.Equal(t, "project description", got.TechnicalDescription.ValueString())
	assert.Equal(t, v1.ProjectPhase_PROJECT_PHASE_ACTIVE.String(), got.DesiredPhase.ValueString())

	require.NotNil(t, got.Initializer)
	require.Len(t, got.Initializer.Specs, 2)
	require.NotNil(t, got.Initializer.Specs[0].ContextURL)
	assert.Equal(t, "https://example.com/context", got.Initializer.Specs[0].ContextURL.URL.ValueString())
	require.NotNil(t, got.Initializer.Specs[1].Git)
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", got.Initializer.Specs[1].Git.RemoteURI.ValueString())

	prebuildGot, diags := projectPrebuildConfigurationModelFromObject(context.Background(), got.PrebuildConfiguration)
	require.False(t, diags.HasError())
	require.NotNil(t, prebuildGot)
	assert.True(t, prebuildGot.Enabled.ValueBool())
	assert.True(t, prebuildGot.EnableJetbrainsWarmup.ValueBool())
	assert.Equal(t, "3600s", prebuildGot.Timeout.ValueString())

	recommendedEditorsGot, diags := projectRecommendedEditorsFromMap(context.Background(), got.RecommendedEditors)
	require.False(t, diags.HasError())
	require.Contains(t, recommendedEditorsGot, "vscode")
	editorVersions := recommendedEditorsGot["vscode"].Versions.Elements()
	require.Len(t, editorVersions, 1)
	version, ok := editorVersions[0].(types.String)
	require.True(t, ok)
	assert.Equal(t, "stable", version.ValueString())

	var metadata projectMetadataModel
	diags = got.Metadata.As(context.Background(), &metadata, basetypes.ObjectAsOptions{})
	require.False(t, diags.HasError())
	assert.Equal(t, "org-123", metadata.OrganizationID.ValueString())

	var usedBy projectUsedByModel
	diags = got.UsedBy.As(context.Background(), &usedBy, basetypes.ObjectAsOptions{})
	require.False(t, diags.HasError())
	assert.Equal(t, int64(1), usedBy.TotalSubjects.ValueInt64())
}

func TestMapProjectToDataSourceModel_ExplicitDisabledPrebuildRemainsPresent(t *testing.T) {
	project := &v1.Project{}
	raw := `{
		"id": "project-789",
		"prebuildConfiguration": {
			"enabled": false
		}
	}`
	require.NoError(t, protojson.Unmarshal([]byte(raw), project))

	got, diags := mapProjectToDataSourceModel(context.Background(), project)
	require.False(t, diags.HasError())

	prebuildGot, diags := projectPrebuildConfigurationModelFromObject(context.Background(), got.PrebuildConfiguration)
	require.False(t, diags.HasError())
	require.NotNil(t, prebuildGot)
	assert.False(t, prebuildGot.Enabled.ValueBool())
}
