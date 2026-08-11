package provider

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &runnerDataSource{}

type runnerDataSource struct {
	client *sdk.Client
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
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
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

	getResp, err := d.client.Services.Runner.GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: config.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.Diagnostics.AddError("Runner not found",
				fmt.Sprintf("No runner found with ID %s", config.ID.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Failed to read runner", err.Error())
		return
	}

	runner := getResp.Msg.GetRunner()
	state := mapRunnerToDataSourceModel(runner)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapRunnerToDataSourceModel(runner *v1.Runner) runnerDataSourceModel {
	m := runnerDataSourceModel{
		ID:           types.StringValue(runner.GetRunnerId()),
		Name:         types.StringValue(runner.GetName()),
		ProviderType: types.StringValue(enumString(runner.GetProvider())),
	}

	m.RunnerManagerID = stringValueOrNull(runner.GetRunnerManagerId())

	m.Spec = &runnerDataSourceSpecModel{
		DesiredPhase:  stringValueOrNull(enumString(runner.GetSpec().GetDesiredPhase())),
		Variant:       stringValueOrNull(enumString(runner.GetSpec().GetVariant())),
		Configuration: mapRunnerConfigToDataSourceModel(runner),
	}

	m.Status = runnerStatusObjectValue(runner.GetStatus())

	return m
}

func mapRunnerConfigToDataSourceModel(runner *v1.Runner) *runnerDataSourceConfigModel {
	configuration := runner.GetSpec().GetConfiguration()
	cfg := &runnerDataSourceConfigModel{
		AutoUpdate:                    types.BoolValue(configuration.GetAutoUpdate()),
		DevcontainerImageCacheEnabled: types.BoolValue(configuration.GetDevcontainerImageCacheEnabled()),
		Region:                        stringValueOrNull(configuration.GetRegion()),
		ReleaseChannel:                stringValueOrNull(enumString(configuration.GetReleaseChannel())),
		LogLevel:                      stringValueOrNull(enumString(configuration.GetLogLevel())),
		Metrics: &runnerDataSourceMetricsModel{
			Enabled:               types.BoolValue(configuration.GetMetrics().GetEnabled()),
			ManagedMetricsEnabled: types.BoolValue(configuration.GetMetrics().GetManagedMetricsEnabled()),
			URL:                   stringValueOrNull(configuration.GetMetrics().GetUrl()),
			Username:              stringValueOrNull(configuration.GetMetrics().GetUsername()),
		},
	}
	if startHour, endHour, ok := mapUpdateWindowValues(configuration.GetUpdateWindow()); ok {
		cfg.UpdateWindow = &runnerDataSourceUpdateWindowModel{
			StartHour: startHour,
			EndHour:   endHour,
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
