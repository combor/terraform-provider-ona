package provider

import (
	"encoding/json"
	"testing"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringValueOrNull(t *testing.T) {
	t.Run("empty string becomes null", func(t *testing.T) {
		got := stringValueOrNull("")
		assert.True(t, got.IsNull())
		assert.False(t, got.IsUnknown())
	})

	t.Run("non-empty string becomes value", func(t *testing.T) {
		got := stringValueOrNull("runner")
		assert.False(t, got.IsNull())
		assert.Equal(t, "runner", got.ValueString())
	})
}

func TestMergeStringWithPrior(t *testing.T) {
	t.Run("non-empty current wins over prior", func(t *testing.T) {
		got := mergeStringWithPrior("new-value", types.StringValue("old-value"))
		assert.Equal(t, "new-value", got.ValueString())
	})

	t.Run("empty current falls back to non-null prior", func(t *testing.T) {
		got := mergeStringWithPrior("", types.StringValue("prior-value"))
		assert.Equal(t, "prior-value", got.ValueString())
	})

	t.Run("empty current returns null when prior is null", func(t *testing.T) {
		got := mergeStringWithPrior("", types.StringNull())
		assert.True(t, got.IsNull())
	})

	t.Run("empty current returns null when prior is unknown", func(t *testing.T) {
		got := mergeStringWithPrior("", types.StringUnknown())
		assert.True(t, got.IsNull())
	})
}

func TestStringListValue(t *testing.T) {
	t.Run("empty slice creates empty list", func(t *testing.T) {
		got := stringListValue([]string{})
		assert.False(t, got.IsNull())
		assert.Empty(t, got.Elements())
	})

	t.Run("populated slice creates correct elements", func(t *testing.T) {
		got := stringListValue([]string{"a", "b", "c"})
		elems := got.Elements()
		require.Len(t, elems, 3)
		v0, ok := elems[0].(types.String)
		require.True(t, ok)
		assert.Equal(t, "a", v0.ValueString())
		v1, ok := elems[1].(types.String)
		require.True(t, ok)
		assert.Equal(t, "b", v1.ValueString())
		v2, ok := elems[2].(types.String)
		require.True(t, ok)
		assert.Equal(t, "c", v2.ValueString())
	})

	t.Run("nil slice creates empty list", func(t *testing.T) {
		got := stringListValue(nil)
		assert.False(t, got.IsNull())
		assert.Empty(t, got.Elements())
	})
}

func TestTimeValueOrNull(t *testing.T) {
	t.Run("zero time returns null", func(t *testing.T) {
		got := timeValueOrNull(time.Time{})
		assert.True(t, got.IsNull())
	})

	t.Run("non-zero time returns RFC3339Nano string", func(t *testing.T) {
		ts := time.Date(2026, time.March, 2, 15, 4, 5, 123456789, time.UTC)
		got := timeValueOrNull(ts)
		assert.Equal(t, "2026-03-02T15:04:05.123456789Z", got.ValueString())
	})
}

func TestBuildConfigParam_HandlesKnownNullAndUnknown(t *testing.T) {
	cfg := &runnerConfigModel{
		AutoUpdate:                    types.BoolUnknown(),
		DevcontainerImageCacheEnabled: types.BoolValue(true),
		Region:                        types.StringNull(),
		ReleaseChannel:                types.StringValue(string(gitpod.RunnerReleaseChannelStable)),
		LogLevel:                      types.StringUnknown(),
		Metrics: &runnerMetricsModel{
			Enabled:  types.BoolValue(true),
			URL:      types.StringNull(),
			Username: types.StringValue("metrics-user"),
			Password: types.StringUnknown(),
		},
	}

	got := buildConfigParam(cfg)

	assert.False(t, got.AutoUpdate.Present)
	assert.True(t, got.DevcontainerImageCacheEnabled.Present)
	assert.Equal(t, true, got.DevcontainerImageCacheEnabled.Value)
	assert.False(t, got.Region.Present)
	assert.True(t, got.ReleaseChannel.Present)
	assert.Equal(t, gitpod.RunnerReleaseChannelStable, got.ReleaseChannel.Value)
	assert.False(t, got.LogLevel.Present)

	require.True(t, got.Metrics.Present)
	assert.True(t, got.Metrics.Value.Enabled.Present)
	assert.Equal(t, true, got.Metrics.Value.Enabled.Value)
	assert.False(t, got.Metrics.Value.URL.Present)
	assert.True(t, got.Metrics.Value.Username.Present)
	assert.Equal(t, "metrics-user", got.Metrics.Value.Username.Value)
	assert.False(t, got.Metrics.Value.Password.Present)
}

func TestBuildRunnerUpdateConfigParam_ClearsUpdateWindowWhenRemovedFromConfiguration(t *testing.T) {
	cfg := &runnerConfigModel{
		AutoUpdate: types.BoolValue(true),
	}
	prior := &runnerConfigModel{
		UpdateWindow: &runnerUpdateWindowModel{
			StartHour: types.Int64Value(22),
		},
	}

	got, present := buildRunnerUpdateConfigParam(cfg, prior)

	assert.True(t, present)
	assert.True(t, got.AutoUpdate.Present)
	assert.Equal(t, true, got.AutoUpdate.Value)
	assert.True(t, got.UpdateWindow.Present)
	assert.False(t, got.UpdateWindow.Value.StartHour.Present)
	assert.False(t, got.UpdateWindow.Value.EndHour.Present)
}

func TestBuildRunnerUpdateParams_ClearsUpdateWindowWhenSpecIsRemoved(t *testing.T) {
	plan := runnerModel{
		ID:   types.StringValue("runner-123"),
		Name: types.StringValue("runner-name"),
	}
	prior := runnerModel{
		Spec: &runnerSpecModel{
			Configuration: &runnerConfigModel{
				UpdateWindow: &runnerUpdateWindowModel{
					StartHour: types.Int64Value(22),
				},
			},
		},
	}

	got := buildRunnerUpdateParams(plan, prior)

	assert.True(t, got.Spec.Present)
	assert.True(t, got.Spec.Value.Configuration.Present)
	assert.True(t, got.Spec.Value.Configuration.Value.UpdateWindow.Present)
	assert.False(t, got.Spec.Value.Configuration.Value.UpdateWindow.Value.StartHour.Present)
	assert.False(t, got.Spec.Value.Configuration.Value.UpdateWindow.Value.EndHour.Present)
}

func TestMapUpdateWindowValues_MissingEndHourRemainsNull(t *testing.T) {
	var window gitpod.UpdateWindow
	require.NoError(t, json.Unmarshal([]byte(`{"startHour":22}`), &window))

	startHour, endHour, ok := mapUpdateWindowValues(window)

	assert.True(t, ok)
	assert.Equal(t, int64(22), startHour.ValueInt64())
	assert.True(t, endHour.IsNull())
}

func TestMapUpdateWindowValues_ExplicitZeroEndHourRemainsPresent(t *testing.T) {
	var window gitpod.UpdateWindow
	require.NoError(t, json.Unmarshal([]byte(`{"startHour":22,"endHour":0}`), &window))

	startHour, endHour, ok := mapUpdateWindowValues(window)

	assert.True(t, ok)
	assert.Equal(t, int64(22), startHour.ValueInt64())
	assert.Equal(t, int64(0), endHour.ValueInt64())
}

func TestMapRunnerToModel_UpdateWindowMissingEndHourRemainsNull(t *testing.T) {
	var cfg gitpod.RunnerConfiguration
	require.NoError(t, json.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true},"updateWindow":{"startHour":22}}`), &cfg))

	prior := runnerModel{
		Spec: &runnerSpecModel{
			Configuration: &runnerConfigModel{},
		},
	}
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

	got := mapRunnerToModel(runner, prior)

	require.NotNil(t, got.Spec)
	require.NotNil(t, got.Spec.Configuration)
	require.NotNil(t, got.Spec.Configuration.UpdateWindow)
	assert.Equal(t, int64(22), got.Spec.Configuration.UpdateWindow.StartHour.ValueInt64())
	assert.True(t, got.Spec.Configuration.UpdateWindow.EndHour.IsNull())
}

func TestMapRunnerToModel_PreservesPriorStateFields(t *testing.T) {
	prior := runnerModel{
		Spec: &runnerSpecModel{
			Configuration: &runnerConfigModel{
				Region: types.StringValue("us-west-2"),
				Metrics: &runnerMetricsModel{
					Password: types.StringValue("secret"),
				},
			},
		},
	}

	runner := gitpod.Runner{
		RunnerID:        "runner-123",
		Name:            "runner-name",
		Provider:        gitpod.RunnerProviderAwsEc2,
		RunnerManagerID: "",
		Spec: gitpod.RunnerSpec{
			DesiredPhase: gitpod.RunnerPhaseActive,
			Variant:      gitpod.RunnerVariantStandard,
			Configuration: gitpod.RunnerConfiguration{
				AutoUpdate:                    true,
				DevcontainerImageCacheEnabled: true,
				ReleaseChannel:                gitpod.RunnerReleaseChannelStable,
				LogLevel:                      gitpod.LogLevelInfo,
				Region:                        "",
				Metrics: gitpod.MetricsConfiguration{
					Enabled:  true,
					URL:      "https://metrics.example",
					Username: "metrics-user",
				},
			},
		},
		Status: gitpod.RunnerStatus{
			Phase:   gitpod.RunnerPhaseDegraded,
			Message: "degraded",
			Version: "1.2.3",
			Region:  "eu-central-1",
		},
	}

	got := mapRunnerToModel(runner, prior)

	assert.Equal(t, "runner-123", got.ID.ValueString())
	assert.Equal(t, "runner-name", got.Name.ValueString())
	assert.Equal(t, string(gitpod.RunnerProviderAwsEc2), got.ProviderType.ValueString())
	assert.True(t, got.RunnerManagerID.IsNull())

	require.NotNil(t, got.Spec)
	assert.Equal(t, string(gitpod.RunnerVariantStandard), got.Spec.Variant.ValueString())
	require.NotNil(t, got.Spec.Configuration)
	assert.Equal(t, true, got.Spec.Configuration.DevcontainerImageCacheEnabled.ValueBool())
	assert.Equal(t, "us-west-2", got.Spec.Configuration.Region.ValueString())

	require.NotNil(t, got.Spec.Configuration.Metrics)
	assert.Equal(t, "secret", got.Spec.Configuration.Metrics.Password.ValueString())
	assert.Equal(t, "https://metrics.example", got.Spec.Configuration.Metrics.URL.ValueString())
	assert.Equal(t, "metrics-user", got.Spec.Configuration.Metrics.Username.ValueString())

	statusAttrs := got.Status.Attributes()
	phase, ok := statusAttrs["phase"].(types.String)
	require.True(t, ok)
	assert.Equal(t, string(gitpod.RunnerPhaseDegraded), phase.ValueString())
}
