package provider

import (
	"context"
	"testing"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapProjectToDataSourceModel_MapsComputedFields(t *testing.T) {
	project := gitpod.Project{
		ID:                   "project-456",
		AutomationsFilePath:  ".gitpod/automations.yaml",
		DevcontainerFilePath: ".devcontainer/devcontainer.json",
		DesiredPhase:         gitpod.ProjectPhaseActive,
		Initializer: gitpod.EnvironmentInitializer{
			Specs: []gitpod.EnvironmentInitializerSpec{{
				ContextURL: gitpod.EnvironmentInitializerSpecsContextURL{
					URL: "https://example.com/context",
				},
				Git: gitpod.EnvironmentInitializerSpecsGit{
					RemoteUri: "https://github.com/combor/terraform-provider-ona",
				},
			}},
		},
		PrebuildConfiguration: gitpod.ProjectPrebuildConfiguration{
			Enabled:               true,
			EnableJetbrainsWarmup: true,
			EnvironmentClassIDs:   []string{"env-1"},
			Timeout:               "3600s",
			Executor: shared.Subject{
				ID:        "subject-1",
				Principal: shared.PrincipalUser,
			},
		},
		RecommendedEditors: gitpod.RecommendedEditors{
			Editors: map[string]gitpod.RecommendedEditorsEditor{
				"vscode": {Versions: []string{"stable"}},
			},
		},
		TechnicalDescription: "project description",
		Metadata: gitpod.ProjectMetadata{
			Name:           "project-name",
			OrganizationID: "org-123",
			CreatedAt:      time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:      time.Date(2026, time.January, 3, 4, 5, 6, 0, time.UTC),
			Creator: shared.Subject{
				ID:        "creator-1",
				Principal: shared.PrincipalUser,
			},
		},
		UsedBy: gitpod.ProjectUsedBy{
			TotalSubjects: 1,
			Subjects: []shared.Subject{{
				ID:        "user-1",
				Principal: shared.PrincipalUser,
			}},
		},
	}

	got, diags := mapProjectToDataSourceModel(context.Background(), project)
	require.False(t, diags.HasError())

	assert.Equal(t, "project-456", got.ID.ValueString())
	assert.Equal(t, "project-name", got.Name.ValueString())
	assert.Equal(t, ".gitpod/automations.yaml", got.AutomationsFilePath.ValueString())
	assert.Equal(t, ".devcontainer/devcontainer.json", got.DevcontainerFilePath.ValueString())
	assert.Equal(t, "project description", got.TechnicalDescription.ValueString())
	assert.Equal(t, string(gitpod.ProjectPhaseActive), got.DesiredPhase.ValueString())

	require.NotNil(t, got.Initializer)
	require.Len(t, got.Initializer.Specs, 1)
	require.NotNil(t, got.Initializer.Specs[0].ContextURL)
	assert.Equal(t, "https://example.com/context", got.Initializer.Specs[0].ContextURL.URL.ValueString())
	require.NotNil(t, got.Initializer.Specs[0].Git)
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", got.Initializer.Specs[0].Git.RemoteURI.ValueString())

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
