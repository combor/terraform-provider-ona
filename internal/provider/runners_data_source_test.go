package provider

import (
	"encoding/json"
	"testing"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRunnersToDataSourceModel_MapsAndSorts(t *testing.T) {
	got := mapRunnersToDataSourceModel([]gitpod.Runner{
		{
			RunnerID:        "runner-b",
			Name:            "Beta",
			Provider:        gitpod.RunnerProviderAwsEc2,
			RunnerManagerID: "mgr-1",
			Status:          gitpod.RunnerStatus{Phase: gitpod.RunnerPhaseActive},
		},
		{
			RunnerID:        "runner-a",
			Name:            "Alpha",
			Provider:        gitpod.RunnerProviderLinuxHost,
			RunnerManagerID: "mgr-2",
			Status:          gitpod.RunnerStatus{Phase: gitpod.RunnerPhaseCreated},
		},
	})

	require.Len(t, got.Runners, 2)

	assert.Equal(t, "runner-a", got.Runners[0].ID.ValueString())
	assert.Equal(t, "Alpha", got.Runners[0].Name.ValueString())
	assert.Equal(t, string(gitpod.RunnerProviderLinuxHost), got.Runners[0].ProviderType.ValueString())
	assert.Equal(t, "mgr-2", got.Runners[0].RunnerManagerID.ValueString())

	assert.Equal(t, "runner-b", got.Runners[1].ID.ValueString())
	assert.Equal(t, "Beta", got.Runners[1].Name.ValueString())
}

func TestMapRunnersToDataSourceModel_EmptyList(t *testing.T) {
	got := mapRunnersToDataSourceModel([]gitpod.Runner{})
	assert.Empty(t, got.Runners)
}

func TestMapRunnersToDataSourceModel_NullRunnerManagerID(t *testing.T) {
	got := mapRunnersToDataSourceModel([]gitpod.Runner{
		{
			RunnerID: "runner-1",
			Name:     "Test",
			Provider: gitpod.RunnerProviderAwsEc2,
		},
	})

	require.Len(t, got.Runners, 1)
	assert.True(t, got.Runners[0].RunnerManagerID.IsNull())
}

func TestMapRunnerToDataSourceModel_UpdateWindowMissingEndHourRemainsNull(t *testing.T) {
	var cfg gitpod.RunnerConfiguration
	require.NoError(t, json.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true},"updateWindow":{"startHour":22}}`), &cfg))

	runner := gitpod.Runner{
		RunnerID: "runner-123",
		Name:     "runner-name",
		Provider: gitpod.RunnerProviderAwsEc2,
		Spec: gitpod.RunnerSpec{
			DesiredPhase:  gitpod.RunnerPhaseActive,
			Variant:       gitpod.RunnerVariantStandard,
			Configuration: cfg,
		},
	}

	got := mapRunnerToDataSourceModel(runner)

	require.NotNil(t, got.Spec)
	require.NotNil(t, got.Spec.Configuration)
	require.NotNil(t, got.Spec.Configuration.UpdateWindow)
	assert.Equal(t, int64(22), got.Spec.Configuration.UpdateWindow.StartHour.ValueInt64())
	assert.True(t, got.Spec.Configuration.UpdateWindow.EndHour.IsNull())
}

func TestMatchesRunnerFilters_NoFilters(t *testing.T) {
	assert.True(t, matchesRunnerFilters(gitpod.Runner{Name: "anything"}, nil))
}

func TestMatchesRunnerFilters_NameMatch(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("name"),
			Values: []types.String{types.StringValue("Alpha"), types.StringValue("Beta")},
		},
	}

	assert.True(t, matchesRunnerFilters(gitpod.Runner{Name: "Alpha"}, filters))
	assert.True(t, matchesRunnerFilters(gitpod.Runner{Name: "Beta"}, filters))
	assert.False(t, matchesRunnerFilters(gitpod.Runner{Name: "Gamma"}, filters))
}

func TestMatchesRunnerFilters_RunnerManagerIDMatch(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("runner_manager_id"),
			Values: []types.String{types.StringValue("mgr-1")},
		},
	}

	assert.True(t, matchesRunnerFilters(gitpod.Runner{RunnerManagerID: "mgr-1"}, filters))
	assert.False(t, matchesRunnerFilters(gitpod.Runner{RunnerManagerID: "mgr-2"}, filters))
}

func TestMatchesRunnerFilters_MultipleFilters(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("name"),
			Values: []types.String{types.StringValue("Alpha")},
		},
		{
			Name:   types.StringValue("runner_manager_id"),
			Values: []types.String{types.StringValue("mgr-1")},
		},
	}

	assert.True(t, matchesRunnerFilters(gitpod.Runner{Name: "Alpha", RunnerManagerID: "mgr-1"}, filters))
	assert.False(t, matchesRunnerFilters(gitpod.Runner{Name: "Alpha", RunnerManagerID: "mgr-2"}, filters))
	assert.False(t, matchesRunnerFilters(gitpod.Runner{Name: "Beta", RunnerManagerID: "mgr-1"}, filters))
}

func TestMatchesRunnerFilters_UnsupportedFilter(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("unknown"),
			Values: []types.String{types.StringValue("value")},
		},
	}

	assert.False(t, matchesRunnerFilters(gitpod.Runner{Name: "anything"}, filters))
}
