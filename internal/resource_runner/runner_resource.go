package resource_runner

import (
	"context"
	"errors"
	"net/http"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource                = (*runnerResource)(nil)
	_ resource.ResourceWithImportState = (*runnerResource)(nil)
)

type runnerResource struct {
	client *gitpod.Client
}

func NewRunnerResource() resource.Resource {
	return &runnerResource{}
}

func (r *runnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *runnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a runner on ona.com.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The runner ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the runner.",
			},
			"kind": schema.StringAttribute{
				Required:    true,
				Description: "Kind of the runner (e.g. RUNNER_KIND_LOCAL, RUNNER_KIND_REMOTE, RUNNER_KIND_LOCAL_CONFIGURATION).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"provider_type": schema.StringAttribute{
				Required:    true,
				Description: "Provider type of the runner (e.g. RUNNER_PROVIDER_AWS_EC2, RUNNER_PROVIDER_LINUX_HOST).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"spec": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Runner specification.",
				Attributes: map[string]schema.Attribute{
					"desired_phase": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Desired phase of the runner.",
					},
					"variant": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Variant of the runner.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"configuration": schema.SingleNestedAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Runner configuration.",
						Attributes: map[string]schema.Attribute{
							"auto_update": schema.BoolAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Whether auto-update is enabled.",
							},
							"devcontainer_image_cache_enabled": schema.BoolAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Whether devcontainer image caching is enabled.",
							},
							"log_level": schema.StringAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Log level (e.g. LOG_LEVEL_DEBUG, LOG_LEVEL_INFO, LOG_LEVEL_WARN, LOG_LEVEL_ERROR).",
							},
							"region": schema.StringAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Region for the runner.",
								PlanModifiers: []planmodifier.String{
									stringplanmodifier.RequiresReplace(),
								},
							},
							"release_channel": schema.StringAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Release channel (e.g. RUNNER_RELEASE_CHANNEL_STABLE, RUNNER_RELEASE_CHANNEL_LATEST).",
							},
							"metrics": schema.SingleNestedAttribute{
								Optional:    true,
								Computed:    true,
								Description: "Metrics configuration.",
								Attributes: map[string]schema.Attribute{
									"enabled": schema.BoolAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Whether metrics are enabled.",
									},
									"password": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Sensitive:   true,
										Description: "Metrics password.",
									},
									"url": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Metrics URL.",
									},
									"username": schema.StringAttribute{
										Optional:    true,
										Computed:    true,
										Description: "Metrics username.",
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
						Computed:    true,
						Description: "Current phase of the runner.",
					},
					"message": schema.StringAttribute{
						Computed:    true,
						Description: "Status message.",
					},
					"version": schema.StringAttribute{
						Computed:    true,
						Description: "Current version of the runner.",
					},
					"region": schema.StringAttribute{
						Computed:    true,
						Description: "Region where the runner is running.",
					},
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the runner was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the runner was last updated.",
			},
			"creator_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who created the runner.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *runnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*gitpod.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			"Expected *gitpod.Client.",
		)
		return
	}

	r.client = client
}

func (r *runnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RunnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := mapModelToNewParams(plan)

	result, err := r.client.Runners.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error creating runner", err.Error())
		return
	}

	state := mapRunnerToModel(result.Runner)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading runner", err.Error())
		return
	}

	newState := mapRunnerToModel(result.Runner)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RunnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current state for the ID.
	var state RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	params := mapModelToUpdateParams(plan)

	_, err := r.client.Runners.Update(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Error updating runner", err.Error())
		return
	}

	// Re-read after update since response is interface{}.
	result, err := r.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(plan.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error reading runner after update", err.Error())
		return
	}

	newState := mapRunnerToModel(result.Runner)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RunnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Runners.Delete(ctx, gitpod.RunnerDeleteParams{
		RunnerID: gitpod.F(state.ID.ValueString()),
		Force:    gitpod.F(false),
	})
	if err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting runner", err.Error())
	}
}

func (r *runnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func isNotFound(err error) bool {
	var apiErr *gitpod.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
