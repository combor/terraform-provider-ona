package provider

import (
	"testing"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestMapRunnerToDataSourceModel_MapsAllFields(t *testing.T) {
	cfg := &v1.RunnerConfiguration{}
	require.NoError(t, protojson.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"region":"us-east-1","releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true,"managedMetricsEnabled":true,"url":"https://metrics.example","username":"metrics-user"},"updateWindow":{"startHour":8,"endHour":12}}`), cfg))

	runner := &v1.Runner{
		RunnerId:        "runner-123",
		Name:            "Test Runner",
		Provider:        v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		RunnerManagerId: "mgr-456",
		Spec: &v1.RunnerSpec{
			DesiredPhase:  v1.RunnerPhase_RUNNER_PHASE_ACTIVE,
			Variant:       v1.RunnerVariant_RUNNER_VARIANT_STANDARD,
			Configuration: cfg,
		},
		Status: &v1.RunnerStatus{
			Phase:   v1.RunnerPhase_RUNNER_PHASE_DEGRADED,
			Message: "degraded",
			Version: "1.2.3",
			Region:  "eu-central-1",
		},
	}

	got := mapRunnerToDataSourceModel(runner)

	assert.Equal(t, "runner-123", got.ID.ValueString())
	assert.Equal(t, "Test Runner", got.Name.ValueString())
	assert.Equal(t, v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2.String(), got.ProviderType.ValueString())
	assert.Equal(t, "mgr-456", got.RunnerManagerID.ValueString())

	require.NotNil(t, got.Spec)
	assert.Equal(t, v1.RunnerPhase_RUNNER_PHASE_ACTIVE.String(), got.Spec.DesiredPhase.ValueString())
	assert.Equal(t, v1.RunnerVariant_RUNNER_VARIANT_STANDARD.String(), got.Spec.Variant.ValueString())

	require.NotNil(t, got.Spec.Configuration)
	assert.True(t, got.Spec.Configuration.AutoUpdate.ValueBool())
	assert.True(t, got.Spec.Configuration.DevcontainerImageCacheEnabled.ValueBool())
	assert.Equal(t, "us-east-1", got.Spec.Configuration.Region.ValueString())
	assert.Equal(t, v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE.String(), got.Spec.Configuration.ReleaseChannel.ValueString())
	assert.Equal(t, v1.LogLevel_LOG_LEVEL_INFO.String(), got.Spec.Configuration.LogLevel.ValueString())

	require.NotNil(t, got.Spec.Configuration.Metrics)
	assert.True(t, got.Spec.Configuration.Metrics.Enabled.ValueBool())
	assert.True(t, got.Spec.Configuration.Metrics.ManagedMetricsEnabled.ValueBool())
	assert.Equal(t, "https://metrics.example", got.Spec.Configuration.Metrics.URL.ValueString())
	assert.Equal(t, "metrics-user", got.Spec.Configuration.Metrics.Username.ValueString())

	require.NotNil(t, got.Spec.Configuration.UpdateWindow)
	assert.Equal(t, int64(8), got.Spec.Configuration.UpdateWindow.StartHour.ValueInt64())
	assert.Equal(t, int64(12), got.Spec.Configuration.UpdateWindow.EndHour.ValueInt64())

	statusAttrs := got.Status.Attributes()
	phase, ok := statusAttrs["phase"].(types.String)
	require.True(t, ok)
	assert.Equal(t, v1.RunnerPhase_RUNNER_PHASE_DEGRADED.String(), phase.ValueString())
	message, ok := statusAttrs["message"].(types.String)
	require.True(t, ok)
	assert.Equal(t, "degraded", message.ValueString())
	version, ok := statusAttrs["version"].(types.String)
	require.True(t, ok)
	assert.Equal(t, "1.2.3", version.ValueString())
	region, ok := statusAttrs["region"].(types.String)
	require.True(t, ok)
	assert.Equal(t, "eu-central-1", region.ValueString())
}

func TestMapRunnerToDataSourceModel_NullOptionalFields(t *testing.T) {
	runner := &v1.Runner{
		RunnerId: "runner-456",
		Name:     "Minimal Runner",
		Provider: v1.RunnerProvider_RUNNER_PROVIDER_GCP,
		Spec: &v1.RunnerSpec{
			Configuration: &v1.RunnerConfiguration{},
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
