package provider

import (
	"context"
	"encoding/json"
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

func TestMapProjectPrebuildConfigurationToModel_HourUtcZeroFromAPI(t *testing.T) {
	// Unmarshal from JSON so the SDK metadata correctly marks HourUtc as present.
	var cfg gitpod.ProjectPrebuildConfiguration
	raw := `{"enabled":true,"trigger":{"dailySchedule":{"hourUtc":0}}}`
	require.NoError(t, json.Unmarshal([]byte(raw), &cfg))

	got := mapProjectPrebuildConfigurationToModel(cfg, nil)

	require.NotNil(t, got)
	require.NotNil(t, got.Trigger)
	require.NotNil(t, got.Trigger.DailySchedule)
	assert.Equal(t, int64(0), got.Trigger.DailySchedule.HourUTC.ValueInt64())
}

func TestBuildEnvironmentInitializerParam_NilInitializer(t *testing.T) {
	_, diags := buildEnvironmentInitializerParam(nil)
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "at least one entry")
}

func TestBuildEnvironmentInitializerParam_EmptySpecs(t *testing.T) {
	_, diags := buildEnvironmentInitializerParam(&projectInitializerModel{Specs: []projectInitializerSpecModel{}})
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "at least one entry")
}

func TestBuildEnvironmentInitializerParam_SpecMissingContextURLAndGit(t *testing.T) {
	_, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{{}},
	})
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "context_url or git")
}

func TestBuildEnvironmentInitializerParam_GitSpecWithAllFields(t *testing.T) {
	got, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			Git: &projectInitializerGitModel{
				RemoteURI:         types.StringValue("https://github.com/combor/terraform-provider-ona"),
				CloneTarget:       types.StringValue("main"),
				TargetMode:        types.StringValue(string(gitpod.EnvironmentInitializerSpecsGitTargetModeCloneTargetModeRemoteBranch)),
				CheckoutLocation:  types.StringValue("src/provider"),
				UpstreamRemoteURI: types.StringValue("https://github.com/upstream/repo"),
			},
		}},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.Specs.Value, 1)

	spec := got.Specs.Value[0]
	assert.False(t, spec.ContextURL.Present)
	assert.True(t, spec.Git.Present)
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", spec.Git.Value.RemoteUri.Value)
	assert.Equal(t, "main", spec.Git.Value.CloneTarget.Value)
	assert.Equal(t, gitpod.EnvironmentInitializerSpecsGitTargetMode("CLONE_TARGET_MODE_REMOTE_BRANCH"), spec.Git.Value.TargetMode.Value)
	assert.Equal(t, "src/provider", spec.Git.Value.CheckoutLocation.Value)
	assert.Equal(t, "https://github.com/upstream/repo", spec.Git.Value.UpstreamRemoteUri.Value)
}

func TestBuildEnvironmentInitializerParam_GitSpecOmitsNullFields(t *testing.T) {
	got, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			Git: &projectInitializerGitModel{
				RemoteURI:         types.StringValue("https://github.com/combor/terraform-provider-ona"),
				CloneTarget:       types.StringNull(),
				TargetMode:        types.StringNull(),
				CheckoutLocation:  types.StringNull(),
				UpstreamRemoteURI: types.StringNull(),
			},
		}},
	})
	require.False(t, diags.HasError())

	spec := got.Specs.Value[0]
	assert.True(t, spec.Git.Value.RemoteUri.Present)
	assert.False(t, spec.Git.Value.CloneTarget.Present)
	assert.False(t, spec.Git.Value.TargetMode.Present)
	assert.False(t, spec.Git.Value.CheckoutLocation.Present)
	assert.False(t, spec.Git.Value.UpstreamRemoteUri.Present)
}

func TestBuildEnvironmentInitializerParam_ContextURLSpec(t *testing.T) {
	got, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			ContextURL: &projectInitializerContextURLModel{
				URL: types.StringValue("https://github.com/combor/terraform-provider-ona"),
			},
		}},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.Specs.Value, 1)

	spec := got.Specs.Value[0]
	assert.True(t, spec.ContextURL.Present)
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", spec.ContextURL.Value.URL.Value)
	assert.False(t, spec.Git.Present)
}

func TestBuildEnvironmentInitializerParam_MultipleSpecs(t *testing.T) {
	got, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{
			{
				Git: &projectInitializerGitModel{
					RemoteURI: types.StringValue("https://github.com/combor/repo-a"),
				},
			},
			{
				ContextURL: &projectInitializerContextURLModel{
					URL: types.StringValue("https://github.com/combor/repo-b"),
				},
				Git: &projectInitializerGitModel{
					RemoteURI: types.StringValue("https://github.com/combor/repo-b"),
				},
			},
		},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.Specs.Value, 2)

	assert.True(t, got.Specs.Value[0].Git.Present)
	assert.False(t, got.Specs.Value[0].ContextURL.Present)

	assert.True(t, got.Specs.Value[1].Git.Present)
	assert.True(t, got.Specs.Value[1].ContextURL.Present)
}

func TestBuildEnvironmentInitializerParam_MixedValidAndInvalidSpecs(t *testing.T) {
	_, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{
			{
				Git: &projectInitializerGitModel{
					RemoteURI: types.StringValue("https://github.com/combor/repo-a"),
				},
			},
			{}, // invalid: no context_url or git
		},
	})
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "specs[1]")
}

func TestMapProjectToModel_DoesNotPreserveRecommendedEditorsWhenPriorIsNull(t *testing.T) {
	prior := projectModel{
		Name:               types.StringValue("project-name"),
		RecommendedEditors: types.MapNull(projectRecommendedEditorObjectType()),
	}

	project := gitpod.Project{
		ID: "project-123",
		Metadata: gitpod.ProjectMetadata{
			Name: "project-name",
		},
		RecommendedEditors: gitpod.RecommendedEditors{},
	}

	got, diags := mapProjectToModel(context.Background(), project, prior)
	require.False(t, diags.HasError())
	assert.True(t, got.RecommendedEditors.IsNull())
}
