package provider

import (
	"encoding/json"
	"testing"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRunnerToDataSourceModel_MapsAllFields(t *testing.T) {
	var cfg gitpod.RunnerConfiguration
	require.NoError(t, json.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"region":"us-east-1","releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true,"managedMetricsEnabled":true,"url":"https://metrics.example","username":"metrics-user"},"updateWindow":{"startHour":8,"endHour":12}}`), &cfg))

	runner := gitpod.Runner{
		RunnerID:        "runner-123",
		Name:            "Test Runner",
		Provider:        gitpod.RunnerProviderAwsEc2,
		RunnerManagerID: "mgr-456",
		Spec: gitpod.RunnerSpec{
			DesiredPhase:  gitpod.RunnerPhaseActive,
			Variant:       gitpod.RunnerVariantStandard,
			Configuration: cfg,
		},
		Status: gitpod.RunnerStatus{
			Phase:   gitpod.RunnerPhaseDegraded,
			Message: "degraded",
			Version: "1.2.3",
			Region:  "eu-central-1",
		},
	}

	got := mapRunnerToDataSourceModel(runner)

	assert.Equal(t, "runner-123", got.ID.ValueString())
	assert.Equal(t, "Test Runner", got.Name.ValueString())
	assert.Equal(t, string(gitpod.RunnerProviderAwsEc2), got.ProviderType.ValueString())
	assert.Equal(t, "mgr-456", got.RunnerManagerID.ValueString())

	require.NotNil(t, got.Spec)
	assert.Equal(t, string(gitpod.RunnerPhaseActive), got.Spec.DesiredPhase.ValueString())
	assert.Equal(t, string(gitpod.RunnerVariantStandard), got.Spec.Variant.ValueString())

	require.NotNil(t, got.Spec.Configuration)
	assert.True(t, got.Spec.Configuration.AutoUpdate.ValueBool())
	assert.True(t, got.Spec.Configuration.DevcontainerImageCacheEnabled.ValueBool())
	assert.Equal(t, "us-east-1", got.Spec.Configuration.Region.ValueString())
	assert.Equal(t, string(gitpod.RunnerReleaseChannelStable), got.Spec.Configuration.ReleaseChannel.ValueString())
	assert.Equal(t, string(gitpod.LogLevelInfo), got.Spec.Configuration.LogLevel.ValueString())

	require.NotNil(t, got.Spec.Configuration.Metrics)
	assert.True(t, got.Spec.Configuration.Metrics.Enabled.ValueBool())
	assert.True(t, got.Spec.Configuration.Metrics.ManagedMetricsEnabled.ValueBool())
	assert.Equal(t, "https://metrics.example", got.Spec.Configuration.Metrics.URL.ValueString())
	assert.Equal(t, "metrics-user", got.Spec.Configuration.Metrics.Username.ValueString())

	require.NotNil(t, got.Spec.Configuration.UpdateWindow)
	assert.Equal(t, int64(8), got.Spec.Configuration.UpdateWindow.StartHour.ValueInt64())
	assert.Equal(t, int64(12), got.Spec.Configuration.UpdateWindow.EndHour.ValueInt64())

	statusAttrs := got.Status.Attributes()
	assert.Equal(t, string(gitpod.RunnerPhaseDegraded), statusAttrs["phase"].(types.String).ValueString())
	assert.Equal(t, "degraded", statusAttrs["message"].(types.String).ValueString())
	assert.Equal(t, "1.2.3", statusAttrs["version"].(types.String).ValueString())
	assert.Equal(t, "eu-central-1", statusAttrs["region"].(types.String).ValueString())
}

func TestMapRunnerToDataSourceModel_NullOptionalFields(t *testing.T) {
	runner := gitpod.Runner{
		RunnerID: "runner-456",
		Name:     "Minimal Runner",
		Provider: gitpod.RunnerProviderLinuxHost,
		Spec: gitpod.RunnerSpec{
			Configuration: gitpod.RunnerConfiguration{},
		},
	}

	got := mapRunnerToDataSourceModel(runner)

	assert.True(t, got.RunnerManagerID.IsNull())

	require.NotNil(t, got.Spec)
	assert.True(t, got.Spec.DesiredPhase.IsNull())
	assert.True(t, got.Spec.Variant.IsNull())

	require.NotNil(t, got.Spec.Configuration)
	assert.True(t, got.Spec.Configuration.Region.IsNull())
	assert.True(t, got.Spec.Configuration.ReleaseChannel.IsNull())
	assert.True(t, got.Spec.Configuration.LogLevel.IsNull())
	assert.Nil(t, got.Spec.Configuration.UpdateWindow)

	require.NotNil(t, got.Spec.Configuration.Metrics)
	assert.True(t, got.Spec.Configuration.Metrics.URL.IsNull())
	assert.True(t, got.Spec.Configuration.Metrics.Username.IsNull())
}

func TestMapRunnerToDataSourceModel_UpdateWindowMissingEndHour(t *testing.T) {
	var cfg gitpod.RunnerConfiguration
	require.NoError(t, json.Unmarshal([]byte(`{"updateWindow":{"startHour":22}}`), &cfg))

	runner := gitpod.Runner{
		RunnerID: "runner-789",
		Name:     "Runner",
		Provider: gitpod.RunnerProviderAwsEc2,
		Spec: gitpod.RunnerSpec{
			Configuration: cfg,
		},
	}

	got := mapRunnerToDataSourceModel(runner)

	require.NotNil(t, got.Spec.Configuration.UpdateWindow)
	assert.Equal(t, int64(22), got.Spec.Configuration.UpdateWindow.StartHour.ValueInt64())
	assert.True(t, got.Spec.Configuration.UpdateWindow.EndHour.IsNull())
}
