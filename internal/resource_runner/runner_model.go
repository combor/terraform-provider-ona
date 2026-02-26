package resource_runner

import (
	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type RunnerModel struct {
	ID           types.String       `tfsdk:"id"`
	Name         types.String       `tfsdk:"name"`
	Kind         types.String       `tfsdk:"kind"`
	ProviderType types.String       `tfsdk:"provider_type"`
	Spec         *RunnerSpecModel   `tfsdk:"spec"`
	Status       *RunnerStatusModel `tfsdk:"status"`
	CreatedAt    types.String       `tfsdk:"created_at"`
	UpdatedAt    types.String       `tfsdk:"updated_at"`
	CreatorID    types.String       `tfsdk:"creator_id"`
}

type RunnerSpecModel struct {
	DesiredPhase  types.String              `tfsdk:"desired_phase"`
	Variant       types.String              `tfsdk:"variant"`
	Configuration *RunnerConfigurationModel `tfsdk:"configuration"`
}

type RunnerConfigurationModel struct {
	AutoUpdate                    types.Bool   `tfsdk:"auto_update"`
	DevcontainerImageCacheEnabled types.Bool   `tfsdk:"devcontainer_image_cache_enabled"`
	LogLevel                      types.String `tfsdk:"log_level"`
	Region                        types.String `tfsdk:"region"`
	ReleaseChannel                types.String `tfsdk:"release_channel"`
	Metrics                       *MetricsModel `tfsdk:"metrics"`
}

type MetricsModel struct {
	Enabled  types.Bool   `tfsdk:"enabled"`
	Password types.String `tfsdk:"password"`
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
}

type RunnerStatusModel struct {
	Phase   types.String `tfsdk:"phase"`
	Message types.String `tfsdk:"message"`
	Version types.String `tfsdk:"version"`
	Region  types.String `tfsdk:"region"`
}

func mapRunnerToModel(r gitpod.Runner) RunnerModel {
	model := RunnerModel{
		ID:           types.StringValue(r.RunnerID),
		Name:         types.StringValue(r.Name),
		Kind:         types.StringValue(string(r.Kind)),
		ProviderType: types.StringValue(string(r.Provider)),
		CreatedAt:    types.StringValue(r.CreatedAt.String()),
		UpdatedAt:    types.StringValue(r.UpdatedAt.String()),
		CreatorID:    types.StringValue(r.Creator.ID),
	}

	model.Spec = &RunnerSpecModel{
		DesiredPhase: types.StringValue(string(r.Spec.DesiredPhase)),
		Variant:      types.StringValue(string(r.Spec.Variant)),
		Configuration: &RunnerConfigurationModel{
			AutoUpdate:                    types.BoolValue(r.Spec.Configuration.AutoUpdate),
			DevcontainerImageCacheEnabled: types.BoolValue(r.Spec.Configuration.DevcontainerImageCacheEnabled),
			LogLevel:                      types.StringValue(string(r.Spec.Configuration.LogLevel)),
			Region:                        types.StringValue(r.Spec.Configuration.Region),
			ReleaseChannel:                types.StringValue(string(r.Spec.Configuration.ReleaseChannel)),
			Metrics: &MetricsModel{
				Enabled:  types.BoolValue(r.Spec.Configuration.Metrics.Enabled),
				Password: types.StringValue(r.Spec.Configuration.Metrics.Password),
				URL:      types.StringValue(r.Spec.Configuration.Metrics.URL),
				Username: types.StringValue(r.Spec.Configuration.Metrics.Username),
			},
		},
	}

	model.Status = &RunnerStatusModel{
		Phase:   types.StringValue(string(r.Status.Phase)),
		Message: types.StringValue(r.Status.Message),
		Version: types.StringValue(r.Status.Version),
		Region:  types.StringValue(r.Status.Region),
	}

	return model
}

func mapModelToNewParams(m RunnerModel) gitpod.RunnerNewParams {
	params := gitpod.RunnerNewParams{
		Name:     gitpod.F(m.Name.ValueString()),
		Kind:     gitpod.F(gitpod.RunnerKind(m.Kind.ValueString())),
		Provider: gitpod.F(gitpod.RunnerProvider(m.ProviderType.ValueString())),
	}

	if m.Spec != nil {
		spec := gitpod.RunnerSpecParam{}

		if !m.Spec.DesiredPhase.IsNull() && !m.Spec.DesiredPhase.IsUnknown() {
			spec.DesiredPhase = gitpod.F(gitpod.RunnerPhase(m.Spec.DesiredPhase.ValueString()))
		}
		if !m.Spec.Variant.IsNull() && !m.Spec.Variant.IsUnknown() {
			spec.Variant = gitpod.F(gitpod.RunnerVariant(m.Spec.Variant.ValueString()))
		}

		if m.Spec.Configuration != nil {
			cfg := gitpod.RunnerConfigurationParam{}
			c := m.Spec.Configuration

			if !c.AutoUpdate.IsNull() && !c.AutoUpdate.IsUnknown() {
				cfg.AutoUpdate = gitpod.F(c.AutoUpdate.ValueBool())
			}
			if !c.DevcontainerImageCacheEnabled.IsNull() && !c.DevcontainerImageCacheEnabled.IsUnknown() {
				cfg.DevcontainerImageCacheEnabled = gitpod.F(c.DevcontainerImageCacheEnabled.ValueBool())
			}
			if !c.LogLevel.IsNull() && !c.LogLevel.IsUnknown() {
				cfg.LogLevel = gitpod.F(gitpod.LogLevel(c.LogLevel.ValueString()))
			}
			if !c.Region.IsNull() && !c.Region.IsUnknown() {
				cfg.Region = gitpod.F(c.Region.ValueString())
			}
			if !c.ReleaseChannel.IsNull() && !c.ReleaseChannel.IsUnknown() {
				cfg.ReleaseChannel = gitpod.F(gitpod.RunnerReleaseChannel(c.ReleaseChannel.ValueString()))
			}
			if c.Metrics != nil {
				met := gitpod.MetricsConfigurationParam{}
				if !c.Metrics.Enabled.IsNull() && !c.Metrics.Enabled.IsUnknown() {
					met.Enabled = gitpod.F(c.Metrics.Enabled.ValueBool())
				}
				if !c.Metrics.Password.IsNull() && !c.Metrics.Password.IsUnknown() {
					met.Password = gitpod.F(c.Metrics.Password.ValueString())
				}
				if !c.Metrics.URL.IsNull() && !c.Metrics.URL.IsUnknown() {
					met.URL = gitpod.F(c.Metrics.URL.ValueString())
				}
				if !c.Metrics.Username.IsNull() && !c.Metrics.Username.IsUnknown() {
					met.Username = gitpod.F(c.Metrics.Username.ValueString())
				}
				cfg.Metrics = gitpod.F(met)
			}

			spec.Configuration = gitpod.F(cfg)
		}

		params.Spec = gitpod.F(spec)
	}

	return params
}

func mapModelToUpdateParams(m RunnerModel) gitpod.RunnerUpdateParams {
	params := gitpod.RunnerUpdateParams{
		RunnerID: gitpod.F(m.ID.ValueString()),
	}

	if !m.Name.IsNull() && !m.Name.IsUnknown() {
		params.Name = gitpod.F(m.Name.ValueString())
	}

	if m.Spec != nil {
		spec := gitpod.RunnerUpdateParamsSpec{}

		if !m.Spec.DesiredPhase.IsNull() && !m.Spec.DesiredPhase.IsUnknown() {
			spec.DesiredPhase = gitpod.F(gitpod.RunnerPhase(m.Spec.DesiredPhase.ValueString()))
		}

		if m.Spec.Configuration != nil {
			cfg := gitpod.RunnerUpdateParamsSpecConfiguration{}
			c := m.Spec.Configuration

			if !c.AutoUpdate.IsNull() && !c.AutoUpdate.IsUnknown() {
				cfg.AutoUpdate = gitpod.F(c.AutoUpdate.ValueBool())
			}
			if !c.DevcontainerImageCacheEnabled.IsNull() && !c.DevcontainerImageCacheEnabled.IsUnknown() {
				cfg.DevcontainerImageCacheEnabled = gitpod.F(c.DevcontainerImageCacheEnabled.ValueBool())
			}
			if !c.LogLevel.IsNull() && !c.LogLevel.IsUnknown() {
				cfg.LogLevel = gitpod.F(gitpod.LogLevel(c.LogLevel.ValueString()))
			}
			if !c.ReleaseChannel.IsNull() && !c.ReleaseChannel.IsUnknown() {
				cfg.ReleaseChannel = gitpod.F(gitpod.RunnerReleaseChannel(c.ReleaseChannel.ValueString()))
			}
			if c.Metrics != nil {
				met := gitpod.RunnerUpdateParamsSpecConfigurationMetrics{}
				if !c.Metrics.Enabled.IsNull() && !c.Metrics.Enabled.IsUnknown() {
					met.Enabled = gitpod.F(c.Metrics.Enabled.ValueBool())
				}
				if !c.Metrics.Password.IsNull() && !c.Metrics.Password.IsUnknown() {
					met.Password = gitpod.F(c.Metrics.Password.ValueString())
				}
				if !c.Metrics.URL.IsNull() && !c.Metrics.URL.IsUnknown() {
					met.URL = gitpod.F(c.Metrics.URL.ValueString())
				}
				if !c.Metrics.Username.IsNull() && !c.Metrics.Username.IsUnknown() {
					met.Username = gitpod.F(c.Metrics.Username.ValueString())
				}
				cfg.Metrics = gitpod.F(met)
			}

			spec.Configuration = gitpod.F(cfg)
		}

		params.Spec = gitpod.F(spec)
	}

	return params
}
