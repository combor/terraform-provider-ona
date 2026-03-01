package provider

import (
	"context"
	"errors"
	"fmt"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &runnerResource{}
	_ resource.ResourceWithImportState = &runnerResource{}
)

type runnerResource struct {
	client *gitpod.Client
}

func NewRunnerResource() resource.Resource {
	return &runnerResource{}
}

// Models

type runnerModel struct {
	ID              types.String     `tfsdk:"id"`
	Name            types.String     `tfsdk:"name"`
	ProviderType    types.String     `tfsdk:"provider_type"`
	RunnerManagerID types.String     `tfsdk:"runner_manager_id"`
	Spec            *runnerSpecModel `tfsdk:"spec"`
	Status          types.Object     `tfsdk:"status"`
}

type runnerSpecModel struct {
	DesiredPhase  types.String       `tfsdk:"desired_phase"`
	Configuration *runnerConfigModel `tfsdk:"configuration"`
}

type runnerConfigModel struct {
	AutoUpdate     types.Bool          `tfsdk:"auto_update"`
	Region         types.String        `tfsdk:"region"`
	ReleaseChannel types.String        `tfsdk:"release_channel"`
	LogLevel       types.String        `tfsdk:"log_level"`
	Metrics        *runnerMetricsModel `tfsdk:"metrics"`
}

type runnerMetricsModel struct {
	Enabled  types.Bool   `tfsdk:"enabled"`
	URL      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (r *runnerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (r *runnerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Gitpod runner.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable runner name.",
			},
			"provider_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner provider type (e.g. `RUNNER_PROVIDER_AWS_EC2`, `RUNNER_PROVIDER_LINUX_HOST`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"runner_manager_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Runner manager ID. Required for managed runners.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"spec": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"desired_phase": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Desired runner phase (e.g. `RUNNER_PHASE_ACTIVE`, `RUNNER_PHASE_INACTIVE`).",
					},
					"configuration": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"auto_update": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether the runner auto-updates.",
								PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
							},
							"region": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Deployment region.",
								PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
							},
							"release_channel": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Release channel (`RUNNER_RELEASE_CHANNEL_STABLE`, `RUNNER_RELEASE_CHANNEL_LATEST`).",
							},
							"log_level": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Log level (`LOG_LEVEL_DEBUG`, `LOG_LEVEL_INFO`, `LOG_LEVEL_WARN`, `LOG_LEVEL_ERROR`).",
							},
							"metrics": schema.SingleNestedAttribute{
								Optional: true,
								Attributes: map[string]schema.Attribute{
									"enabled": schema.BoolAttribute{
										Optional: true,
									},
									"url": schema.StringAttribute{
										Optional: true,
									},
									"username": schema.StringAttribute{
										Optional: true,
									},
									"password": schema.StringAttribute{
										Optional:  true,
										Sensitive: true,
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

func (r *runnerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*gitpod.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *gitpod.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *runnerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := gitpod.RunnerNewParams{
		Name:     gitpod.F(plan.Name.ValueString()),
		Provider: gitpod.F(gitpod.RunnerProvider(plan.ProviderType.ValueString())),
	}
	if !plan.RunnerManagerID.IsNull() && !plan.RunnerManagerID.IsUnknown() && plan.RunnerManagerID.ValueString() != "" {
		params.RunnerManagerID = gitpod.F(plan.RunnerManagerID.ValueString())
	}
	if plan.Spec != nil {
		params.Spec = gitpod.F(buildSpecParam(plan.Spec))
	}

	createResp, err := r.client.Runners.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create runner", err.Error())
		return
	}

	// Read back to get computed fields
	getResp, err := r.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(createResp.Runner.RunnerID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read runner after create", err.Error())
		return
	}

	state := mapRunnerToModel(getResp.Runner, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read runner", err.Error())
		return
	}

	newState := mapRunnerToModel(getResp.Runner, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := gitpod.RunnerUpdateParams{
		RunnerID: gitpod.F(plan.ID.ValueString()),
		Name:     gitpod.F(plan.Name.ValueString()),
	}
	if plan.Spec != nil {
		params.Spec = gitpod.F(buildUpdateSpecParam(plan.Spec))
	}

	_, err := r.client.Runners.Update(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update runner", err.Error())
		return
	}

	// Read back to get computed fields
	getResp, err := r.client.Runners.Get(ctx, gitpod.RunnerGetParams{
		RunnerID: gitpod.F(plan.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read runner after update", err.Error())
		return
	}

	state := mapRunnerToModel(getResp.Runner, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Runners.Delete(ctx, gitpod.RunnerDeleteParams{
		RunnerID: gitpod.F(state.ID.ValueString()),
		Force:    gitpod.F(true),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete runner", err.Error())
	}
}

func (r *runnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers

func buildSpecParam(spec *runnerSpecModel) gitpod.RunnerSpecParam {
	p := gitpod.RunnerSpecParam{}
	if !spec.DesiredPhase.IsNull() && !spec.DesiredPhase.IsUnknown() {
		p.DesiredPhase = gitpod.F(gitpod.RunnerPhase(spec.DesiredPhase.ValueString()))
	}
	if spec.Configuration != nil {
		p.Configuration = gitpod.F(buildConfigParam(spec.Configuration))
	}
	return p
}

func buildConfigParam(cfg *runnerConfigModel) gitpod.RunnerConfigurationParam {
	p := gitpod.RunnerConfigurationParam{}
	if !cfg.AutoUpdate.IsNull() && !cfg.AutoUpdate.IsUnknown() {
		p.AutoUpdate = gitpod.F(cfg.AutoUpdate.ValueBool())
	}
	if !cfg.Region.IsNull() && !cfg.Region.IsUnknown() {
		p.Region = gitpod.F(cfg.Region.ValueString())
	}
	if !cfg.ReleaseChannel.IsNull() && !cfg.ReleaseChannel.IsUnknown() {
		p.ReleaseChannel = gitpod.F(gitpod.RunnerReleaseChannel(cfg.ReleaseChannel.ValueString()))
	}
	if !cfg.LogLevel.IsNull() && !cfg.LogLevel.IsUnknown() {
		p.LogLevel = gitpod.F(gitpod.LogLevel(cfg.LogLevel.ValueString()))
	}
	if cfg.Metrics != nil {
		p.Metrics = gitpod.F(buildMetricsParam(cfg.Metrics))
	}
	return p
}

func buildMetricsParam(m *runnerMetricsModel) gitpod.MetricsConfigurationParam {
	p := gitpod.MetricsConfigurationParam{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p.Enabled = gitpod.F(m.Enabled.ValueBool())
	}
	if !m.URL.IsNull() && !m.URL.IsUnknown() {
		p.URL = gitpod.F(m.URL.ValueString())
	}
	if !m.Username.IsNull() && !m.Username.IsUnknown() {
		p.Username = gitpod.F(m.Username.ValueString())
	}
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p.Password = gitpod.F(m.Password.ValueString())
	}
	return p
}

func buildUpdateSpecParam(spec *runnerSpecModel) gitpod.RunnerUpdateParamsSpec {
	p := gitpod.RunnerUpdateParamsSpec{}
	if !spec.DesiredPhase.IsNull() && !spec.DesiredPhase.IsUnknown() {
		p.DesiredPhase = gitpod.F(gitpod.RunnerPhase(spec.DesiredPhase.ValueString()))
	}
	if spec.Configuration != nil {
		p.Configuration = gitpod.F(buildUpdateConfigParam(spec.Configuration))
	}
	return p
}

func buildUpdateConfigParam(cfg *runnerConfigModel) gitpod.RunnerUpdateParamsSpecConfiguration {
	p := gitpod.RunnerUpdateParamsSpecConfiguration{}
	if !cfg.AutoUpdate.IsNull() && !cfg.AutoUpdate.IsUnknown() {
		p.AutoUpdate = gitpod.F(cfg.AutoUpdate.ValueBool())
	}
	if !cfg.ReleaseChannel.IsNull() && !cfg.ReleaseChannel.IsUnknown() {
		p.ReleaseChannel = gitpod.F(gitpod.RunnerReleaseChannel(cfg.ReleaseChannel.ValueString()))
	}
	if !cfg.LogLevel.IsNull() && !cfg.LogLevel.IsUnknown() {
		p.LogLevel = gitpod.F(gitpod.LogLevel(cfg.LogLevel.ValueString()))
	}
	if cfg.Metrics != nil {
		p.Metrics = gitpod.F(buildUpdateMetricsParam(cfg.Metrics))
	}
	return p
}

func buildUpdateMetricsParam(m *runnerMetricsModel) gitpod.RunnerUpdateParamsSpecConfigurationMetrics {
	p := gitpod.RunnerUpdateParamsSpecConfigurationMetrics{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p.Enabled = gitpod.F(m.Enabled.ValueBool())
	}
	if !m.URL.IsNull() && !m.URL.IsUnknown() {
		p.URL = gitpod.F(m.URL.ValueString())
	}
	if !m.Username.IsNull() && !m.Username.IsUnknown() {
		p.Username = gitpod.F(m.Username.ValueString())
	}
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p.Password = gitpod.F(m.Password.ValueString())
	}
	return p
}

func mapRunnerToModel(runner gitpod.Runner, prior runnerModel) runnerModel {
	m := runnerModel{
		ID:           types.StringValue(runner.RunnerID),
		Name:         types.StringValue(runner.Name),
		ProviderType: types.StringValue(string(runner.Provider)),
	}

	if runner.RunnerManagerID != "" {
		m.RunnerManagerID = types.StringValue(runner.RunnerManagerID)
	} else {
		m.RunnerManagerID = types.StringNull()
	}

	// Map spec — preserve user-set values the API doesn't return
	if prior.Spec != nil {
		spec := &runnerSpecModel{
			DesiredPhase: types.StringValue(string(runner.Spec.DesiredPhase)),
		}
		if prior.Spec.Configuration != nil {
			cfg := &runnerConfigModel{
				AutoUpdate:     types.BoolValue(runner.Spec.Configuration.AutoUpdate),
				ReleaseChannel: types.StringValue(string(runner.Spec.Configuration.ReleaseChannel)),
				LogLevel:       types.StringValue(string(runner.Spec.Configuration.LogLevel)),
			}
			if runner.Spec.Configuration.Region != "" {
				cfg.Region = types.StringValue(runner.Spec.Configuration.Region)
			} else if !prior.Spec.Configuration.Region.IsNull() {
				cfg.Region = prior.Spec.Configuration.Region
			} else {
				cfg.Region = types.StringNull()
			}
			if prior.Spec.Configuration.Metrics != nil {
				cfg.Metrics = &runnerMetricsModel{
					Enabled:  types.BoolValue(runner.Spec.Configuration.Metrics.Enabled),
					URL:      stringValueOrNull(runner.Spec.Configuration.Metrics.URL),
					Username: stringValueOrNull(runner.Spec.Configuration.Metrics.Username),
					// Preserve password from prior state — API doesn't return it
					Password: prior.Spec.Configuration.Metrics.Password,
				}
			}
			spec.Configuration = cfg
		}
		m.Spec = spec
	}

	// Map status as types.Object
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

func stringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
