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

func TestBuildProjectPrebuildConfigurationParam_HandlesKnownNullAndUnknown(t *testing.T) {
	cfg := &projectPrebuildConfigurationModel{
		Enabled:               types.BoolValue(true),
		EnableJetbrainsWarmup: types.BoolUnknown(),
		EnvironmentClassIDs:   stringListValue([]string{"env-1", "env-2"}),
		Executor: &projectSubjectModel{
			ID:        types.StringValue("subject-1"),
			Principal: types.StringValue(v1.Principal_PRINCIPAL_USER.String()),
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

	assert.True(t, got.GetEnabled())
	// unknown values are never sent; proto3 bools have no presence, so this is false
	assert.False(t, got.GetEnableJetbrainsWarmup())
	assert.Equal(t, []string{"env-1", "env-2"}, got.GetEnvironmentClassIds())
	require.NotNil(t, got.GetExecutor())
	assert.Equal(t, "subject-1", got.GetExecutor().GetId())
	assert.Equal(t, v1.Principal_PRINCIPAL_USER, got.GetExecutor().GetPrincipal())
	assert.Nil(t, got.GetTimeout())
	require.NotNil(t, got.GetTrigger())
	assert.Equal(t, int32(2), got.GetTrigger().GetDailySchedule().GetHourUtc())
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

	assert.Empty(t, got.GetEditors()["vscode"].GetVersions())
	assert.Equal(t, []string{"2025.1"}, got.GetEditors()["goland"].GetVersions())
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

	project := &v1.Project{
		Id:                   "project-123",
		AutomationsFilePath:  "",
		DevcontainerFilePath: "",
		DesiredPhase:         v1.ProjectPhase_PROJECT_PHASE_DELETED,
		Initializer: &v1.EnvironmentInitializer{
			Specs: []*v1.EnvironmentInitializer_Spec{{
				Spec: &v1.EnvironmentInitializer_Spec_Git{
					Git: &v1.GitInitializer{
						RemoteUri: "https://github.com/combor/terraform-provider-ona",
					},
				},
			}},
		},
		Metadata: &v1.ProjectMetadata{
			Name:           "project-name",
			OrganizationId: "org-123",
			CreatedAt:      timestamppb.New(time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)),
			UpdatedAt:      timestamppb.New(time.Date(2026, time.January, 3, 4, 5, 6, 0, time.UTC)),
			Creator: &v1.Subject{
				Id:        "creator-1",
				Principal: v1.Principal_PRINCIPAL_USER,
			},
		},
		RecommendedEditors:   &v1.RecommendedEditors{},
		TechnicalDescription: "",
		UsedBy: &v1.Project_UsedBy{
			TotalSubjects: 1,
			Subjects: []*v1.Subject{{
				Id:        "user-1",
				Principal: v1.Principal_PRINCIPAL_USER,
			}},
		},
	}

	got, diags := mapProjectToModel(context.Background(), project, prior)
	require.False(t, diags.HasError())

	assert.Equal(t, "project-123", got.ID.ValueString())
	assert.Equal(t, ".gitpod/automations.yaml", got.AutomationsFilePath.ValueString())
	assert.Equal(t, ".devcontainer/devcontainer.json", got.DevcontainerFilePath.ValueString())
	assert.Equal(t, "deep project description", got.TechnicalDescription.ValueString())
	assert.Equal(t, v1.ProjectPhase_PROJECT_PHASE_DELETED.String(), got.DesiredPhase.ValueString())

	prebuildGot, diags := projectPrebuildConfigurationModelFromObject(context.Background(), got.PrebuildConfiguration)
	require.False(t, diags.HasError())
	require.NotNil(t, prebuildGot)
	assert.False(t, prebuildGot.Enabled.ValueBool())
	assert.False(t, prebuildGot.EnableJetbrainsWarmup.ValueBool())
	prebuildEnvClassIDs := prebuildGot.EnvironmentClassIDs.Elements()
	require.Len(t, prebuildEnvClassIDs, 1)
	envClassID, ok := prebuildEnvClassIDs[0].(types.String)
	require.True(t, ok)
	assert.Equal(t, "env-1", envClassID.ValueString())
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
	// Unmarshal from the wire format so hourUtc:0 arrives inside a present trigger.
	cfg := &v1.ProjectPrebuildConfiguration{}
	raw := `{"enabled":true,"trigger":{"dailySchedule":{"hourUtc":0}}}`
	require.NoError(t, protojson.Unmarshal([]byte(raw), cfg))

	got := mapProjectPrebuildConfigurationToModel(cfg, nil)

	require.NotNil(t, got)
	require.NotNil(t, got.Trigger)
	require.NotNil(t, got.Trigger.DailySchedule)
	assert.Equal(t, int64(0), got.Trigger.DailySchedule.HourUTC.ValueInt64())
}

func TestMapProjectPrebuildConfigurationToModel_EnabledFalseFromAPI(t *testing.T) {
	cfg := &v1.ProjectPrebuildConfiguration{}
	raw := `{"enabled":false}`
	require.NoError(t, protojson.Unmarshal([]byte(raw), cfg))

	got := mapProjectPrebuildConfigurationToModel(cfg, nil)

	require.NotNil(t, got)
	assert.False(t, got.Enabled.ValueBool())
	// enable_jetbrains_warmup is a plain proto3 bool: absent on the wire is
	// indistinguishable from false, so it lands as false rather than null.
	assert.False(t, got.EnableJetbrainsWarmup.ValueBool())
	assert.True(t, got.EnvironmentClassIDs.IsNull())
	assert.Nil(t, got.Executor)
	assert.True(t, got.Timeout.IsNull())
	assert.Nil(t, got.Trigger)
}

func TestMapProjectPrebuildConfigurationToModel_DoesNotReuseUnknownPriorValues(t *testing.T) {
	cfg := &v1.ProjectPrebuildConfiguration{}
	raw := `{"enabled":true}`
	require.NoError(t, protojson.Unmarshal([]byte(raw), cfg))

	got := mapProjectPrebuildConfigurationToModel(cfg, &projectPrebuildConfigurationModel{
		Enabled:               types.BoolValue(true),
		EnableJetbrainsWarmup: types.BoolUnknown(),
		EnvironmentClassIDs:   types.ListUnknown(types.StringType),
		Executor: &projectSubjectModel{
			ID:        types.StringUnknown(),
			Principal: types.StringUnknown(),
		},
		Timeout: types.StringUnknown(),
		Trigger: &projectPrebuildTriggerModel{
			DailySchedule: &projectPrebuildDailyScheduleModel{
				HourUTC: types.Int64Unknown(),
			},
		},
	})

	require.NotNil(t, got)
	assert.True(t, got.Enabled.ValueBool())
	// see above: no presence for proto3 bools, so the unknown prior is not reused
	assert.False(t, got.EnableJetbrainsWarmup.ValueBool())
	assert.True(t, got.EnvironmentClassIDs.IsNull())
	assert.Nil(t, got.Executor)
	assert.True(t, got.Timeout.IsNull())
	assert.Nil(t, got.Trigger)
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
				TargetMode:        types.StringValue(v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_BRANCH.String()),
				CheckoutLocation:  types.StringValue("src/provider"),
				UpstreamRemoteURI: types.StringValue("https://github.com/upstream/repo"),
			},
		}},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.GetSpecs(), 1)

	spec := got.GetSpecs()[0]
	assert.Nil(t, spec.GetContextUrl())
	require.NotNil(t, spec.GetGit())
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", spec.GetGit().GetRemoteUri())
	assert.Equal(t, "main", spec.GetGit().GetCloneTarget())
	assert.Equal(t, v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_BRANCH, spec.GetGit().GetTargetMode())
	assert.Equal(t, "src/provider", spec.GetGit().GetCheckoutLocation())
	assert.Equal(t, "https://github.com/upstream/repo", spec.GetGit().GetUpstreamRemoteUri())
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

	spec := got.GetSpecs()[0]
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", spec.GetGit().GetRemoteUri())
	assert.Empty(t, spec.GetGit().GetCloneTarget())
	assert.Equal(t, v1.GitInitializer_CLONE_TARGET_MODE_UNSPECIFIED, spec.GetGit().GetTargetMode())
	assert.Empty(t, spec.GetGit().GetCheckoutLocation())
	assert.Empty(t, spec.GetGit().GetUpstreamRemoteUri())
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
	require.Len(t, got.GetSpecs(), 1)

	spec := got.GetSpecs()[0]
	require.NotNil(t, spec.GetContextUrl())
	assert.Equal(t, "https://github.com/combor/terraform-provider-ona", spec.GetContextUrl().GetUrl())
	assert.Nil(t, spec.GetGit())
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
			},
		},
	})
	require.False(t, diags.HasError())
	require.Len(t, got.GetSpecs(), 2)

	require.NotNil(t, got.GetSpecs()[0].GetGit())
	assert.Nil(t, got.GetSpecs()[0].GetContextUrl())

	assert.Nil(t, got.GetSpecs()[1].GetGit())
	require.NotNil(t, got.GetSpecs()[1].GetContextUrl())
}

func TestBuildEnvironmentInitializerParam_SpecWithBothContextURLAndGit(t *testing.T) {
	_, diags := buildEnvironmentInitializerParam(&projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			ContextURL: &projectInitializerContextURLModel{
				URL: types.StringValue("https://github.com/combor/repo-b"),
			},
			Git: &projectInitializerGitModel{
				RemoteURI: types.StringValue("https://github.com/combor/repo-b"),
			},
		}},
	})
	require.True(t, diags.HasError())
	assert.Contains(t, diags.Errors()[0].Detail(), "exactly one of context_url or git")
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

	project := &v1.Project{
		Id: "project-123",
		Metadata: &v1.ProjectMetadata{
			Name: "project-name",
		},
		RecommendedEditors: &v1.RecommendedEditors{},
	}

	got, diags := mapProjectToModel(context.Background(), project, prior)
	require.False(t, diags.HasError())
	assert.True(t, got.RecommendedEditors.IsNull())
}

// Drift from context_url to git must not leave both variants in state: the
// oneof cannot express that and buildEnvironmentInitializerParam rejects it.
func TestMapProjectInitializerToModel_DropsInactiveVariantAfterDrift(t *testing.T) {
	prior := &projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			ContextURL: &projectInitializerContextURLModel{
				URL: types.StringValue("https://example.com/context"),
			},
		}},
	}

	got := mapProjectInitializerToModel(&v1.EnvironmentInitializer{
		Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{
				Git: &v1.GitInitializer{RemoteUri: "https://github.com/combor/repo"},
			},
		}},
	}, prior)

	require.NotNil(t, got)
	require.Len(t, got.Specs, 1)
	assert.Nil(t, got.Specs[0].ContextURL)
	require.NotNil(t, got.Specs[0].Git)
	assert.Equal(t, "https://github.com/combor/repo", got.Specs[0].Git.RemoteURI.ValueString())

	// The resulting state must round-trip back into a valid request.
	_, diags := buildEnvironmentInitializerParam(got)
	assert.False(t, diags.HasError())
}

func TestMapProjectInitializerToModel_KeepsPriorFieldsWithinActiveVariant(t *testing.T) {
	prior := &projectInitializerModel{
		Specs: []projectInitializerSpecModel{{
			Git: &projectInitializerGitModel{
				RemoteURI:        types.StringValue("https://github.com/combor/repo"),
				CheckoutLocation: types.StringValue("src/provider"),
			},
		}},
	}

	got := mapProjectInitializerToModel(&v1.EnvironmentInitializer{
		Specs: []*v1.EnvironmentInitializer_Spec{{
			Spec: &v1.EnvironmentInitializer_Spec_Git{
				Git: &v1.GitInitializer{RemoteUri: "https://github.com/combor/repo"},
			},
		}},
	}, prior)

	require.NotNil(t, got.Specs[0].Git)
	assert.Equal(t, "src/provider", got.Specs[0].Git.CheckoutLocation.ValueString())
}
