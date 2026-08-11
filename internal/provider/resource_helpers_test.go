package provider

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UpdateRunner rejects desiredPhase with a failed_precondition, so the provider
// must never send it, not even when state carries a phase from a previous read.
func TestBuildRunnerUpdateParams_NeverSendsDesiredPhase(t *testing.T) {
	prior := runnerModel{
		Spec: &runnerSpecModel{DesiredPhase: types.StringValue(v1.RunnerPhase_RUNNER_PHASE_ACTIVE.String())},
	}

	t.Run("omitted alongside a configuration change", func(t *testing.T) {
		plan := runnerModel{
			ID:           types.StringValue("runner-123"),
			Name:         types.StringValue("runner-name"),
			ProviderType: types.StringValue(v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2.String()),
			Spec: &runnerSpecModel{
				DesiredPhase:  types.StringValue(v1.RunnerPhase_RUNNER_PHASE_INACTIVE.String()),
				Configuration: &runnerConfigModel{AutoUpdate: types.BoolValue(true)},
			},
		}

		var diags diag.Diagnostics
		got := buildRunnerUpdateParams(plan, prior, &diags)
		require.False(t, diags.HasError())

		require.NotNil(t, got.Spec)
		assert.Nil(t, got.Spec.DesiredPhase)
	})

	t.Run("phase alone leaves the spec out entirely", func(t *testing.T) {
		plan := runnerModel{
			ID:           types.StringValue("runner-123"),
			Name:         types.StringValue("runner-name"),
			ProviderType: types.StringValue(v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2.String()),
			Spec: &runnerSpecModel{
				DesiredPhase: types.StringValue(v1.RunnerPhase_RUNNER_PHASE_INACTIVE.String()),
			},
		}

		var diags diag.Diagnostics
		got := buildRunnerUpdateParams(plan, prior, &diags)
		require.False(t, diags.HasError())

		assert.Nil(t, got.Spec)
		require.NotNil(t, got.Name)
	})
}

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
	t.Run("nil timestamp returns null", func(t *testing.T) {
		got := timeValueOrNull(nil)
		assert.True(t, got.IsNull())
	})

	t.Run("zero time returns null", func(t *testing.T) {
		got := timeValueOrNull(timestamppb.New(time.Time{}))
		assert.True(t, got.IsNull())
	})

	t.Run("non-zero time returns RFC3339Nano string", func(t *testing.T) {
		ts := timestamppb.New(time.Date(2026, time.March, 2, 15, 4, 5, 123456789, time.UTC))
		got := timeValueOrNull(ts)
		assert.Equal(t, "2026-03-02T15:04:05.123456789Z", got.ValueString())
	})
}

func TestBuildConfigParam_HandlesKnownNullAndUnknown(t *testing.T) {
	cfg := &runnerConfigModel{
		AutoUpdate:                    types.BoolUnknown(),
		DevcontainerImageCacheEnabled: types.BoolValue(true),
		Region:                        types.StringNull(),
		ReleaseChannel:                types.StringValue(v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE.String()),
		LogLevel:                      types.StringUnknown(),
		Metrics: &runnerMetricsModel{
			Enabled:  types.BoolValue(true),
			URL:      types.StringNull(),
			Username: types.StringValue("metrics-user"),
			Password: types.StringUnknown(),
		},
	}

	var diags diag.Diagnostics
	got := buildConfigParam(cfg, &diags)
	require.False(t, diags.HasError())

	// unknown and null values are never sent; proto3 scalars have no presence,
	// so they stay at their zero value.
	assert.False(t, got.GetAutoUpdate())
	assert.True(t, got.GetDevcontainerImageCacheEnabled())
	assert.Empty(t, got.GetRegion())
	assert.Equal(t, v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE, got.GetReleaseChannel())
	assert.Equal(t, v1.LogLevel_LOG_LEVEL_UNSPECIFIED, got.GetLogLevel())

	require.NotNil(t, got.GetMetrics())
	assert.True(t, got.GetMetrics().GetEnabled())
	assert.Empty(t, got.GetMetrics().GetUrl())
	assert.Equal(t, "metrics-user", got.GetMetrics().GetUsername())
	assert.Empty(t, got.GetMetrics().GetPassword())
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

	var diags diag.Diagnostics
	got, present := buildRunnerUpdateConfigParam(cfg, prior, &diags)
	require.False(t, diags.HasError())

	assert.True(t, present)
	require.NotNil(t, got.AutoUpdate)
	assert.True(t, got.GetAutoUpdate())
	require.NotNil(t, got.GetUpdateWindow())
	assert.Nil(t, got.GetUpdateWindow().StartHour)
	assert.Nil(t, got.GetUpdateWindow().EndHour)
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

	var diags diag.Diagnostics
	got := buildRunnerUpdateParams(plan, prior, &diags)
	require.False(t, diags.HasError())

	require.NotNil(t, got.GetSpec())
	require.NotNil(t, got.GetSpec().GetConfiguration())
	require.NotNil(t, got.GetSpec().GetConfiguration().GetUpdateWindow())
	assert.Nil(t, got.GetSpec().GetConfiguration().GetUpdateWindow().StartHour)
	assert.Nil(t, got.GetSpec().GetConfiguration().GetUpdateWindow().EndHour)
}

func TestMapUpdateWindowValues_MissingEndHourRemainsNull(t *testing.T) {
	window := &v1.UpdateWindow{}
	require.NoError(t, protojson.Unmarshal([]byte(`{"startHour":22}`), window))

	startHour, endHour, ok := mapUpdateWindowValues(window)

	assert.True(t, ok)
	assert.Equal(t, int64(22), startHour.ValueInt64())
	assert.True(t, endHour.IsNull())
}

func TestMapUpdateWindowValues_ExplicitZeroEndHourRemainsPresent(t *testing.T) {
	window := &v1.UpdateWindow{}
	require.NoError(t, protojson.Unmarshal([]byte(`{"startHour":22,"endHour":0}`), window))

	startHour, endHour, ok := mapUpdateWindowValues(window)

	assert.True(t, ok)
	assert.Equal(t, int64(22), startHour.ValueInt64())
	assert.Equal(t, int64(0), endHour.ValueInt64())
}

func TestMapRunnerToModel_UpdateWindowMissingEndHourRemainsNull(t *testing.T) {
	cfg := &v1.RunnerConfiguration{}
	require.NoError(t, protojson.Unmarshal([]byte(`{"autoUpdate":true,"devcontainerImageCacheEnabled":true,"releaseChannel":"RUNNER_RELEASE_CHANNEL_STABLE","logLevel":"LOG_LEVEL_INFO","metrics":{"enabled":true},"updateWindow":{"startHour":22}}`), cfg))

	prior := runnerModel{
		Spec: &runnerSpecModel{
			Configuration: &runnerConfigModel{},
		},
	}
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

	runner := &v1.Runner{
		RunnerId:        "runner-123",
		Name:            "runner-name",
		Provider:        v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2,
		RunnerManagerId: "",
		Spec: &v1.RunnerSpec{
			DesiredPhase: v1.RunnerPhase_RUNNER_PHASE_ACTIVE,
			Variant:      v1.RunnerVariant_RUNNER_VARIANT_STANDARD,
			Configuration: &v1.RunnerConfiguration{
				AutoUpdate:                    true,
				DevcontainerImageCacheEnabled: true,
				ReleaseChannel:                v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE,
				LogLevel:                      v1.LogLevel_LOG_LEVEL_INFO,
				Region:                        "",
				Metrics: &v1.MetricsConfiguration{
					Enabled:  true,
					Url:      "https://metrics.example",
					Username: "metrics-user",
				},
			},
		},
		Status: &v1.RunnerStatus{
			Phase:   v1.RunnerPhase_RUNNER_PHASE_DEGRADED,
			Message: "degraded",
			Version: "1.2.3",
			Region:  "eu-central-1",
		},
	}

	got := mapRunnerToModel(runner, prior)

	assert.Equal(t, "runner-123", got.ID.ValueString())
	assert.Equal(t, "runner-name", got.Name.ValueString())
	assert.Equal(t, v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2.String(), got.ProviderType.ValueString())
	assert.True(t, got.RunnerManagerID.IsNull())

	require.NotNil(t, got.Spec)
	assert.Equal(t, v1.RunnerVariant_RUNNER_VARIANT_STANDARD.String(), got.Spec.Variant.ValueString())
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
	assert.Equal(t, v1.RunnerPhase_RUNNER_PHASE_DEGRADED.String(), phase.ValueString())
}

func TestEnumValue_RejectsUnknownName(t *testing.T) {
	var diags diag.Diagnostics

	got := enumValue[v1.RunnerProvider]("provider_type", "RUNNER_PROVIDER_TYPO", v1.RunnerProvider_value, &diags)

	require.True(t, diags.HasError())
	assert.Equal(t, v1.RunnerProvider_RUNNER_PROVIDER_UNSPECIFIED, got)
	assert.Contains(t, diags.Errors()[0].Detail(), "RUNNER_PROVIDER_TYPO")
	assert.Contains(t, diags.Errors()[0].Detail(), "RUNNER_PROVIDER_AWS_EC2")
}

func TestEnumValue_AcceptsKnownName(t *testing.T) {
	var diags diag.Diagnostics

	got := enumValue[v1.RunnerProvider]("provider_type", "RUNNER_PROVIDER_AWS_EC2", v1.RunnerProvider_value, &diags)

	require.False(t, diags.HasError())
	assert.Equal(t, v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2, got)
}

func TestDurationString(t *testing.T) {
	assert.Equal(t, "", durationString(nil))
	assert.Equal(t, "3600s", durationString(durationpb.New(time.Hour)))
	assert.Equal(t, "1.5s", durationString(durationpb.New(1500*time.Millisecond)))
}

func TestTimeoutValueWithPrior(t *testing.T) {
	t.Run("keeps an equivalent configured spelling", func(t *testing.T) {
		got := timeoutValueWithPrior(durationpb.New(time.Hour), types.StringValue("1h"))
		assert.Equal(t, "1h", got.ValueString())
	})

	t.Run("uses the API spelling when the duration differs", func(t *testing.T) {
		got := timeoutValueWithPrior(durationpb.New(time.Hour), types.StringValue("30m"))
		assert.Equal(t, "3600s", got.ValueString())
	})

	t.Run("uses the API spelling when prior is null", func(t *testing.T) {
		got := timeoutValueWithPrior(durationpb.New(time.Hour), types.StringNull())
		assert.Equal(t, "3600s", got.ValueString())
	})

	t.Run("uses the API spelling when prior is unparseable", func(t *testing.T) {
		got := timeoutValueWithPrior(durationpb.New(time.Hour), types.StringValue("not-a-duration"))
		assert.Equal(t, "3600s", got.ValueString())
	})
}

func TestEnumValue_RejectsUnspecifiedName(t *testing.T) {
	var diags diag.Diagnostics

	got := enumValue[v1.RunnerReleaseChannel]("release_channel",
		v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_UNSPECIFIED.String(),
		v1.RunnerReleaseChannel_value, &diags)

	require.True(t, diags.HasError())
	assert.Equal(t, v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_UNSPECIFIED, got)
}

func TestValidatedHour(t *testing.T) {
	t.Run("accepts the documented range", func(t *testing.T) {
		for _, hour := range []int64{0, 12, 23} {
			var diags diag.Diagnostics
			got, ok := validatedHour("start_hour", types.Int64Value(hour), &diags)
			require.True(t, ok)
			require.False(t, diags.HasError())
			assert.Equal(t, hour, got)
		}
	})

	t.Run("rejects out-of-range values that would wrap when narrowed", func(t *testing.T) {
		for _, hour := range []int64{-1, 24, 4294967296} {
			var diags diag.Diagnostics
			_, ok := validatedHour("hour_utc", types.Int64Value(hour), &diags)
			assert.False(t, ok)
			assert.True(t, diags.HasError())
		}
	})
}

func TestBuildUpdateWindowParam_RejectsOutOfRangeHours(t *testing.T) {
	var diags diag.Diagnostics

	// 2^32 narrows to 0 as a uint32, which would look like a valid hour.
	got := buildUpdateWindowParam(&runnerUpdateWindowModel{
		StartHour: types.Int64Value(4294967296),
	}, &diags)

	assert.True(t, diags.HasError())
	assert.Nil(t, got.StartHour)
}
