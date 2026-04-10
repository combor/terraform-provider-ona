package provider

import (
	"context"
	"errors"
	"fmt"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &runnerDataSource{}

type runnerDataSource struct {
	client *gitpod.Client
}

func NewRunnerDataSource() datasource.DataSource {
	return &runnerDataSource{}
}

func (d *runnerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (d *runnerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Gitpod runner by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable runner name.",
			},
			"provider_type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner provider type.",
			},
			"runner_manager_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner manager ID.",
			},
			"spec": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"desired_phase": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Desired runner phase.",
					},
					"variant": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Runner variant.",
					},
					"configuration": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"auto_update": schema.BoolAttribute{
								Computed:            true,
								MarkdownDescription: "Whether the runner auto-updates.",
							},
							"devcontainer_image_cache_enabled": schema.BoolAttribute{
								Computed:            true,
								MarkdownDescription: "Whether the devcontainer build cache is enabled.",
							},
							"region": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Deployment region.",
							},
							"release_channel": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Release channel.",
							},
							"log_level": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Log level.",
							},
							"metrics": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"enabled":                 schema.BoolAttribute{Computed: true},
									"managed_metrics_enabled": schema.BoolAttribute{Computed: true, MarkdownDescription: "When true, the runner pushes metrics to the management plane instead of directly to the remote_write endpoint."},
									"url":                     schema.StringAttribute{Computed: true},
									"username":                schema.StringAttribute{Computed: true},
								},
							},
							"update_window": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Daily time window (UTC) during which auto-updates are allowed.",
								Attributes: map[string]schema.Attribute{
									"start_hour": schema.Int64Attribute{
										Computed:            true,
										MarkdownDescription: "Start of the update window as a UTC hour (0-23).",
									},
									"end_hour": schema.Int64Attribute{
										Computed:            true,
										MarkdownDescription: "End of the update window as a UTC hour (0-23).",
									},
								},
							},
						},
					},
				},
			},
			"status": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"phase":   schema.StringAttribute{Computed: true},
					"message": schema.StringAttribute{Computed: true},
					"version": schema.StringAttribute{Computed: true},
					"region":  schema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func (d *runnerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*gitpod.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *gitpod.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *runnerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config runnerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(config.ID.ValueString()),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.Diagnostics.AddError("Runner not found",
				fmt.Sprintf("No runner found with ID %s", config.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read runner", err.Error())
		return
	}

	runner := getResp.Runner
	state := mapRunnerToDataSourceModel(runner)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapRunnerToDataSourceModel(runner gitpod.Runner) runnerDataSourceModel {
	m := runnerDataSourceModel{
		ID:           types.StringValue(runner.RunnerID),
		Name:         types.StringValue(runner.Name),
		ProviderType: types.StringValue(string(runner.Provider)),
	}

	if runner.RunnerManagerID != "" {
		m.RunnerManagerID = types.StringValue(runner.RunnerManagerID)
	} else {
		m.RunnerManagerID = types.StringNull()
	}

	m.Spec = &runnerDataSourceSpecModel{
		DesiredPhase:  stringValueOrNull(string(runner.Spec.DesiredPhase)),
		Variant:       stringValueOrNull(string(runner.Spec.Variant)),
		Configuration: mapRunnerConfigToDataSourceModel(runner),
	}

	statusAttrTypes := map[string]attr.Type{
		"phase":   types.StringType,
		"message": types.StringType,
		"version": types.StringType,
		"region":  types.StringType,
	}
	statusValues := map[string]attr.Value{
		"phase":   types.StringValue(string(runner.Status.Phase)),
		"message": types.StringValue(runner.Status.Message),
		"version": types.StringValue(runner.Status.Version),
		"region":  types.StringValue(runner.Status.Region),
	}
	m.Status, _ = types.ObjectValue(statusAttrTypes, statusValues)

	return m
}

func mapRunnerConfigToDataSourceModel(runner gitpod.Runner) *runnerDataSourceConfigModel {
	cfg := &runnerDataSourceConfigModel{
		AutoUpdate:                    types.BoolValue(runner.Spec.Configuration.AutoUpdate),
		DevcontainerImageCacheEnabled: types.BoolValue(runner.Spec.Configuration.DevcontainerImageCacheEnabled),
		Region:                        stringValueOrNull(runner.Spec.Configuration.Region),
		ReleaseChannel:                stringValueOrNull(string(runner.Spec.Configuration.ReleaseChannel)),
		LogLevel:                      stringValueOrNull(string(runner.Spec.Configuration.LogLevel)),
		Metrics: &runnerDataSourceMetricsModel{
			Enabled:               types.BoolValue(runner.Spec.Configuration.Metrics.Enabled),
			ManagedMetricsEnabled: types.BoolValue(runner.Spec.Configuration.Metrics.ManagedMetricsEnabled),
			URL:                   stringValueOrNull(runner.Spec.Configuration.Metrics.URL),
			Username:              stringValueOrNull(runner.Spec.Configuration.Metrics.Username),
		},
	}
	if runner.Spec.Configuration.UpdateWindow.JSON.RawJSON() != "" {
		cfg.UpdateWindow = &runnerDataSourceUpdateWindowModel{
			StartHour: types.Int64Value(runner.Spec.Configuration.UpdateWindow.StartHour),
			EndHour:   types.Int64Value(runner.Spec.Configuration.UpdateWindow.EndHour),
		}
	}
	return cfg
}

// Data source models — separate from resource models since all fields
// are computed and there's no password field (API doesn't return it).

type runnerDataSourceModel struct {
	ID              types.String               `tfsdk:"id"`
	Name            types.String               `tfsdk:"name"`
	ProviderType    types.String               `tfsdk:"provider_type"`
	RunnerManagerID types.String               `tfsdk:"runner_manager_id"`
	Spec            *runnerDataSourceSpecModel `tfsdk:"spec"`
	Status          types.Object               `tfsdk:"status"`
}

type runnerDataSourceSpecModel struct {
	DesiredPhase  types.String                 `tfsdk:"desired_phase"`
	Variant       types.String                 `tfsdk:"variant"`
	Configuration *runnerDataSourceConfigModel `tfsdk:"configuration"`
}

type runnerDataSourceConfigModel struct {
	AutoUpdate                    types.Bool                         `tfsdk:"auto_update"`
	DevcontainerImageCacheEnabled types.Bool                         `tfsdk:"devcontainer_image_cache_enabled"`
	Region                        types.String                       `tfsdk:"region"`
	ReleaseChannel                types.String                       `tfsdk:"release_channel"`
	LogLevel                      types.String                       `tfsdk:"log_level"`
	Metrics                       *runnerDataSourceMetricsModel      `tfsdk:"metrics"`
	UpdateWindow                  *runnerDataSourceUpdateWindowModel `tfsdk:"update_window"`
}

type runnerDataSourceUpdateWindowModel struct {
	StartHour types.Int64 `tfsdk:"start_hour"`
	EndHour   types.Int64 `tfsdk:"end_hour"`
}

type runnerDataSourceMetricsModel struct {
	Enabled               types.Bool   `tfsdk:"enabled"`
	ManagedMetricsEnabled types.Bool   `tfsdk:"managed_metrics_enabled"`
	URL                   types.String `tfsdk:"url"`
	Username              types.String `tfsdk:"username"`
}
