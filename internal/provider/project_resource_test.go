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

func TestBuildProjectPrebuildConfigurationParam_HandlesKnownNullAndUnknown(t *testing.T) {
	cfg := &projectPrebuildConfigurationModel{
		Enabled:               types.BoolValue(true),
		EnableJetbrainsWarmup: types.BoolUnknown(),
		EnvironmentClassIDs:   stringListValue([]string{"env-1", "env-2"}),
		Executor: &projectSubjectModel{
			ID:        types.StringValue("subject-1"),
			Principal: types.StringValue(string(shared.PrincipalUser)),
		},
		Timeout: types.StringNull(),
		Trigger: &projectPrebuildTriggerModel{
			DailySchedule: &projectPrebuildDailyScheduleModel{
				HourUTC: types.Int64Value(2),
			},
		},
	}

	got, diags := buildProjectPrebuildConfigurationParam(context.Background(), cfg)
	require.False(t, diags.HasError())

	assert.True(t, got.Enabled.Present)
	assert.Equal(t, true, got.Enabled.Value)
	assert.False(t, got.EnableJetbrainsWarmup.Present)
	assert.True(t, got.EnvironmentClassIDs.Present)
	assert.Equal(t, []string{"env-1", "env-2"}, got.EnvironmentClassIDs.Value)
	assert.True(t, got.Executor.Present)
	assert.Equal(t, "subject-1", got.Executor.Value.ID.Value)
	assert.Equal(t, shared.PrincipalUser, got.Executor.Value.Principal.Value)
	assert.False(t, got.Timeout.Present)
	assert.True(t, got.Trigger.Present)
	assert.Equal(t, int64(2), got.Trigger.Value.DailySchedule.Value.HourUtc.Value)
}

func TestBuildRecommendedEditorsParam_EmptyVersionsRemainExplicit(t *testing.T) {
	editors := map[string]projectRecommendedEditor{
		"vscode": {
			Versions: stringListValue([]string{}),
		},
		"goland": {
			Versions: stringListValue([]string{"2025.1"}),
		},
	}

	got, diags := buildRecommendedEditorsParam(context.Background(), editors)
	require.False(t, diags.HasError())

	assert.True(t, got.Editors.Present)
	assert.Empty(t, got.Editors.Value["vscode"].Versions.Value)
	assert.Equal(t, []string{"2025.1"}, got.Editors.Value["goland"].Versions.Value)
}

func TestMapProjectToModel_PreservesOmittedFieldsFromPriorState(t *testing.T) {
	prebuildPrior, diags := projectPrebuildConfigurationObjectValue(context.Background(), &projectPrebuildConfigurationModel{
		Enabled:               types.BoolValue(false),
		EnableJetbrainsWarmup: types.BoolValue(false),
		EnvironmentClassIDs:   stringListValue([]string{"env-1"}),
		Trigger: &projectPrebuildTriggerModel{
			DailySchedule: &projectPrebuildDailyScheduleModel{
				HourUTC: types.Int64Value(0),
			},
		},
	})
	require.False(t, diags.HasError())

	recommendedEditorsPrior, diags := projectRecommendedEditorsMapValue(context.Background(), map[string]projectRecommendedEditor{
		"vscode": {
			Versions: stringListValue([]string{"stable"}),
		},
	})
	require.False(t, diags.HasError())

	prior := projectModel{
		Name:                  types.StringValue("project-name"),
		AutomationsFilePath:   types.StringValue(".gitpod/automations.yaml"),
		DevcontainerFilePath:  types.StringValue(".devcontainer/devcontainer.json"),
		TechnicalDescription:  types.StringValue("deep project description"),
		PrebuildConfiguration: prebuildPrior,
		RecommendedEditors:    recommendedEditorsPrior,
	}

	project := gitpod.Project{
		ID:                   "project-123",
		AutomationsFilePath:  "",
		DevcontainerFilePath: "",
		DesiredPhase:         gitpod.ProjectPhaseDeleted,
		Initializer: gitpod.EnvironmentInitializer{
			Specs: []gitpod.EnvironmentInitializerSpec{{
				Git: gitpod.EnvironmentInitializerSpecsGit{
					RemoteUri: "https://github.com/combor/terraform-provider-ona",
				},
			}},
		},
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
		RecommendedEditors:   gitpod.RecommendedEditors{},
		TechnicalDescription: "",
		UsedBy: gitpod.ProjectUsedBy{
			TotalSubjects: 1,
			Subjects: []shared.Subject{{
				ID:        "user-1",
				Principal: shared.PrincipalUser,
			}},
		},
	}

	got, diags := mapProjectToModel(context.Background(), project, prior)
	require.False(t, diags.HasError())

	assert.Equal(t, "project-123", got.ID.ValueString())
	assert.Equal(t, ".gitpod/automations.yaml", got.AutomationsFilePath.ValueString())
	assert.Equal(t, ".devcontainer/devcontainer.json", got.DevcontainerFilePath.ValueString())
	assert.Equal(t, "deep project description", got.TechnicalDescription.ValueString())
	assert.Equal(t, string(gitpod.ProjectPhaseDeleted), got.DesiredPhase.ValueString())

	prebuildGot, diags := projectPrebuildConfigurationModelFromObject(context.Background(), got.PrebuildConfiguration)
	require.False(t, diags.HasError())
	require.NotNil(t, prebuildGot)
	assert.Equal(t, int64(0), prebuildGot.Trigger.DailySchedule.HourUTC.ValueInt64())

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
