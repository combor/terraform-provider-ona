package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestMapRunnersToDataSourceModel_MapsAndSorts(t *testing.T) {
	got := mapRunnersToDataSourceModel([]*v1.Runner{
		{
			RunnerId:        "runner-b",
			Name:            "Beta",
			Provider:        v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
			RunnerManagerId: "mgr-1",
			Status:          &v1.RunnerStatus{Phase: v1.RunnerPhase_RUNNER_PHASE_ACTIVE},
		},
		{
			RunnerId:        "runner-a",
			Name:            "Alpha",
			Provider:        v1.RunnerProvider_RUNNER_PROVIDER_GCP,
			RunnerManagerId: "mgr-2",
			Status:          &v1.RunnerStatus{Phase: v1.RunnerPhase_RUNNER_PHASE_CREATED},
		},
	})

	require.Len(t, got.Runners, 2)

	assert.Equal(t, "runner-a", got.Runners[0].ID.ValueString())
	assert.Equal(t, "Alpha", got.Runners[0].Name.ValueString())
	assert.Equal(t, v1.RunnerProvider_RUNNER_PROVIDER_GCP.String(), got.Runners[0].ProviderType.ValueString())
	assert.Equal(t, "mgr-2", got.Runners[0].RunnerManagerID.ValueString())

	assert.Equal(t, "runner-b", got.Runners[1].ID.ValueString())
	assert.Equal(t, "Beta", got.Runners[1].Name.ValueString())
}

func TestMapRunnersToDataSourceModel_EmptyList(t *testing.T) {
	got := mapRunnersToDataSourceModel([]*v1.Runner{})
	assert.Empty(t, got.Runners)
}

func TestMapRunnersToDataSourceModel_NullRunnerManagerID(t *testing.T) {
	got := mapRunnersToDataSourceModel([]*v1.Runner{
		{
			RunnerId: "runner-1",
			Name:     "Test",
			Provider: v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		},
	})

	require.Len(t, got.Runners, 1)
	assert.True(t, got.Runners[0].RunnerManagerID.IsNull())
}

func TestMapRunnerToDataSourceModel_UpdateWindowMissingEndHourRemainsNull(t *testing.T) {
	cfg := &v1.RunnerConfiguration{}
	require.NoError(t, protojson.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true},"updateWindow":{"startHour":22}}`), cfg))

	runner := &v1.Runner{
		RunnerId: "runner-123",
		Name:     "runner-name",
		Provider: v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		Spec: &v1.RunnerSpec{
			DesiredPhase:  v1.RunnerPhase_RUNNER_PHASE_ACTIVE,
			Variant:       v1.RunnerVariant_RUNNER_VARIANT_STANDARD,
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
	assert.True(t, matchesRunnerFilters(&v1.Runner{Name: "anything"}, nil))
}

func TestMatchesRunnerFilters_NameMatch(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("name"),
			Values: []types.String{types.StringValue("Alpha"), types.StringValue("Beta")},
		},
	}

	assert.True(t, matchesRunnerFilters(&v1.Runner{Name: "Alpha"}, filters))
	assert.True(t, matchesRunnerFilters(&v1.Runner{Name: "Beta"}, filters))
	assert.False(t, matchesRunnerFilters(&v1.Runner{Name: "Gamma"}, filters))
}

func TestMatchesRunnerFilters_RunnerManagerIDMatch(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("runner_manager_id"),
			Values: []types.String{types.StringValue("mgr-1")},
		},
	}

	assert.True(t, matchesRunnerFilters(&v1.Runner{RunnerManagerId: "mgr-1"}, filters))
	assert.False(t, matchesRunnerFilters(&v1.Runner{RunnerManagerId: "mgr-2"}, filters))
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

	assert.True(t, matchesRunnerFilters(&v1.Runner{Name: "Alpha", RunnerManagerId: "mgr-1"}, filters))
	assert.False(t, matchesRunnerFilters(&v1.Runner{Name: "Alpha", RunnerManagerId: "mgr-2"}, filters))
	assert.False(t, matchesRunnerFilters(&v1.Runner{Name: "Beta", RunnerManagerId: "mgr-1"}, filters))
}

func TestMatchesRunnerFilters_UnsupportedFilter(t *testing.T) {
	filters := []runnersFilterModel{
		{
			Name:   types.StringValue("unknown"),
			Values: []types.String{types.StringValue("value")},
		},
	}

	assert.False(t, matchesRunnerFilters(&v1.Runner{Name: "anything"}, filters))
}
