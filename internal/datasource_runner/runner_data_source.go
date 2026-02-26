package datasource_runner

import (
	"context"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = (*runnerDataSource)(nil)

type runnerDataSource struct {
	client *gitpod.Client
}

type runnerDataSourceModel struct {
	ID           types.String            `tfsdk:"id"`
	Name         types.String            `tfsdk:"name"`
	Kind         types.String            `tfsdk:"kind"`
	ProviderType types.String            `tfsdk:"provider_type"`
	Spec         *runnerSpecModel        `tfsdk:"spec"`
	Status       *runnerStatusModel      `tfsdk:"status"`
	CreatedAt    types.String            `tfsdk:"created_at"`
	UpdatedAt    types.String            `tfsdk:"updated_at"`
	CreatorID    types.String            `tfsdk:"creator_id"`
}

type runnerSpecModel struct {
	DesiredPhase  types.String                 `tfsdk:"desired_phase"`
	Variant       types.String                 `tfsdk:"variant"`
	Configuration *runnerConfigurationModel    `tfsdk:"configuration"`
}

type runnerConfigurationModel struct {
	AutoUpdate                    types.Bool   `tfsdk:"auto_update"`
	DevcontainerImageCacheEnabled types.Bool   `tfsdk:"devcontainer_image_cache_enabled"`
	LogLevel                      types.String `tfsdk:"log_level"`
	Region                        types.String `tfsdk:"region"`
	ReleaseChannel                types.String `tfsdk:"release_channel"`
	Metrics                       *metricsModel `tfsdk:"metrics"`
}

type metricsModel struct {
	Enabled  types.Bool   `tfsdk:"enabled"`
	Password types.String `tfsdk:"password"`
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
}

type runnerStatusModel struct {
	Phase   types.String `tfsdk:"phase"`
	Message types.String `tfsdk:"message"`
	Version types.String `tfsdk:"version"`
	Region  types.String `tfsdk:"region"`
}

func NewRunnerDataSource() datasource.DataSource {
	return &runnerDataSource{}
}

func (d *runnerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (d *runnerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a runner from ona.com.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "The runner ID.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the runner.",
			},
			"kind": schema.StringAttribute{
				Computed:    true,
				Description: "Kind of the runner.",
			},
			"provider_type": schema.StringAttribute{
				Computed:    true,
				Description: "Provider type of the runner.",
			},
			"spec": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Runner specification.",
				Attributes: map[string]schema.Attribute{
					"desired_phase": schema.StringAttribute{
						Computed: true,
					},
					"variant": schema.StringAttribute{
						Computed: true,
					},
					"configuration": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"auto_update": schema.BoolAttribute{
								Computed: true,
							},
							"devcontainer_image_cache_enabled": schema.BoolAttribute{
								Computed: true,
							},
							"log_level": schema.StringAttribute{
								Computed: true,
							},
							"region": schema.StringAttribute{
								Computed: true,
							},
							"release_channel": schema.StringAttribute{
								Computed: true,
							},
							"metrics": schema.SingleNestedAttribute{
								Computed: true,
								Attributes: map[string]schema.Attribute{
									"enabled": schema.BoolAttribute{
										Computed: true,
									},
									"password": schema.StringAttribute{
										Computed:  true,
										Sensitive: true,
									},
									"url": schema.StringAttribute{
										Computed: true,
									},
									"username": schema.StringAttribute{
										Computed: true,
									},
								},
							},
						},
					},
				},
			},
			"status": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Current status of the runner.",
				Attributes: map[string]schema.Attribute{
					"phase": schema.StringAttribute{
						Computed: true,
					},
					"message": schema.StringAttribute{
						Computed: true,
					},
					"version": schema.StringAttribute{
						Computed: true,
					},
					"region": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the runner was created.",
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the runner was last updated.",
			},
			"creator_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who created the runner.",
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
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			"Expected *gitpod.Client.",
		)
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

	result, err := d.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(config.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading runner", err.Error())
		return
	}

	r := result.Runner
	state := runnerDataSourceModel{
		ID:           types.StringValue(r.RunnerID),
		Name:         types.StringValue(r.Name),
		Kind:         types.StringValue(string(r.Kind)),
		ProviderType: types.StringValue(string(r.Provider)),
		Spec: &runnerSpecModel{
			DesiredPhase: types.StringValue(string(r.Spec.DesiredPhase)),
			Variant:      types.StringValue(string(r.Spec.Variant)),
			Configuration: &runnerConfigurationModel{
				AutoUpdate:                    types.BoolValue(r.Spec.Configuration.AutoUpdate),
				DevcontainerImageCacheEnabled: types.BoolValue(r.Spec.Configuration.DevcontainerImageCacheEnabled),
				LogLevel:                      types.StringValue(string(r.Spec.Configuration.LogLevel)),
				Region:                        types.StringValue(r.Spec.Configuration.Region),
				ReleaseChannel:                types.StringValue(string(r.Spec.Configuration.ReleaseChannel)),
				Metrics: &metricsModel{
					Enabled:  types.BoolValue(r.Spec.Configuration.Metrics.Enabled),
					Password: types.StringValue(r.Spec.Configuration.Metrics.Password),
					URL:      types.StringValue(r.Spec.Configuration.Metrics.URL),
					Username: types.StringValue(r.Spec.Configuration.Metrics.Username),
				},
			},
		},
		Status: &runnerStatusModel{
			Phase:   types.StringValue(string(r.Status.Phase)),
			Message: types.StringValue(r.Status.Message),
			Version: types.StringValue(r.Status.Version),
			Region:  types.StringValue(r.Status.Region),
		},
		CreatedAt: types.StringValue(r.CreatedAt.String()),
		UpdatedAt: types.StringValue(r.UpdatedAt.String()),
		CreatorID: types.StringValue(r.Creator.ID),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
