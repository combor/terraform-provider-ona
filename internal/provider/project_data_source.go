package provider

import (
	"context"
	"fmt"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &projectDataSource{}

type projectDataSource struct {
	client *gitpod.Client
}

func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Look up an existing Gitpod project by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project ID.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Human-readable project name.",
			},
			"automations_file_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Path to the automations file relative to the repository root.",
			},
			"devcontainer_file_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Path to the devcontainer file relative to the repository root.",
			},
			"initializer": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Defines how the project content is initialized.",
				Attributes: map[string]schema.Attribute{
					"specs": schema.ListNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Initializer specs. Each entry may define `context_url`, `git`, or both.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"context_url": schema.SingleNestedAttribute{
									Computed:            true,
									MarkdownDescription: "URL used to initialize the project context.",
									Attributes: map[string]schema.Attribute{
										"url": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Source URL for the context.",
										},
									},
								},
								"git": schema.SingleNestedAttribute{
									Computed:            true,
									MarkdownDescription: "Git repository initializer settings.",
									Attributes: map[string]schema.Attribute{
										"checkout_location": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Relative checkout path inside the environment.",
										},
										"clone_target": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Clone target interpreted according to `target_mode`.",
										},
										"remote_uri": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Git remote URI.",
										},
										"target_mode": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Git clone target mode.",
										},
										"upstream_remote_uri": schema.StringAttribute{
											Computed:            true,
											MarkdownDescription: "Upstream remote URI for fork-based repositories.",
										},
									},
								},
							},
						},
					},
				},
			},
			"prebuild_configuration": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Prebuild configuration for the project.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether prebuilds are enabled.",
					},
					"enable_jetbrains_warmup": schema.BoolAttribute{
						Computed:            true,
						MarkdownDescription: "Whether JetBrains warmup runs during prebuilds.",
					},
					"environment_class_ids": schema.ListAttribute{
						ElementType:         types.StringType,
						Computed:            true,
						MarkdownDescription: "Environment class IDs that should receive prebuilds.",
					},
					"executor": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Subject whose SCM credentials are used for prebuilds.",
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Executor subject ID.",
							},
							"principal": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Executor principal.",
							},
						},
					},
					"timeout": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Maximum prebuild duration, such as `3600s`.",
					},
					"trigger": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "Prebuild trigger settings.",
						Attributes: map[string]schema.Attribute{
							"daily_schedule": schema.SingleNestedAttribute{
								Computed:            true,
								MarkdownDescription: "Daily schedule trigger.",
								Attributes: map[string]schema.Attribute{
									"hour_utc": schema.Int64Attribute{
										Computed:            true,
										MarkdownDescription: "UTC hour (0-23) for the daily prebuild trigger.",
									},
								},
							},
						},
					},
				},
			},
			"recommended_editors": schema.MapNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Recommended editors keyed by editor alias.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"versions": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: "Recommended versions. Use an empty list to recommend all available versions.",
						},
					},
				},
			},
			"technical_description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Detailed technical description of the project.",
			},
			"desired_phase": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Desired lifecycle phase of the project.",
			},
			"metadata": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Project metadata returned by the API.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed: true,
					},
					"organization_id": schema.StringAttribute{
						Computed: true,
					},
					"created_at": schema.StringAttribute{
						Computed: true,
					},
					"updated_at": schema.StringAttribute{
						Computed: true,
					},
					"creator": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Computed: true,
							},
							"principal": schema.StringAttribute{
								Computed: true,
							},
						},
					},
				},
			},
			"used_by": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Summary of recent project usage.",
				Attributes: map[string]schema.Attribute{
					"total_subjects": schema.Int64Attribute{
						Computed: true,
					},
					"subjects": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed: true,
								},
								"principal": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	d.client = client
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := d.client.Projects.Get(ctx, gitpod.ProjectGetParams{
		ProjectID: gitpod.F(config.ID.ValueString()),
	})
	if err != nil {
		if isAPINotFound(err) {
			resp.Diagnostics.AddError("Project not found",
				fmt.Sprintf("No project found with ID %s", config.ID.ValueString()))
			return
		}

		resp.Diagnostics.AddError("Failed to read project", err.Error())
		return
	}

	state, diags := mapProjectToDataSourceModel(ctx, getResp.Project)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapProjectToDataSourceModel(ctx context.Context, project gitpod.Project) (projectModel, diag.Diagnostics) {
	return mapProjectToModel(ctx, project, projectModel{})
}
