package provider

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"google.golang.org/protobuf/proto"
)

var (
	_ resource.Resource                = &runnerResource{}
	_ resource.ResourceWithImportState = &runnerResource{}
)

type runnerResource struct {
	client *sdk.Client
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
	Variant       types.String       `tfsdk:"variant"`
	Configuration *runnerConfigModel `tfsdk:"configuration"`
}

type runnerConfigModel struct {
	AutoUpdate                    types.Bool               `tfsdk:"auto_update"`
	DevcontainerImageCacheEnabled types.Bool               `tfsdk:"devcontainer_image_cache_enabled"`
	Region                        types.String             `tfsdk:"region"`
	ReleaseChannel                types.String             `tfsdk:"release_channel"`
	LogLevel                      types.String             `tfsdk:"log_level"`
	Metrics                       *runnerMetricsModel      `tfsdk:"metrics"`
	UpdateWindow                  *runnerUpdateWindowModel `tfsdk:"update_window"`
}

type runnerUpdateWindowModel struct {
	StartHour types.Int64 `tfsdk:"start_hour"`
	EndHour   types.Int64 `tfsdk:"end_hour"`
}

type runnerMetricsModel struct {
	Enabled               types.Bool   `tfsdk:"enabled"`
	ManagedMetricsEnabled types.Bool   `tfsdk:"managed_metrics_enabled"`
	URL                   types.String `tfsdk:"url"`
	Username              types.String `tfsdk:"username"`
	Password              types.String `tfsdk:"password"`
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
				MarkdownDescription: "Runner manager ID. Required for managed runners. Find it in [ona.com](https://ona.com) → Settings → Runners → ⋯ → Copy runner manager ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"spec": schema.SingleNestedAttribute{
				Optional: true,
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"desired_phase": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Desired runner phase reported by the API (e.g. `RUNNER_PHASE_ACTIVE`). Read-only: `UpdateRunner` rejects `desiredPhase`, which only ever applied to local runners as an organization-wide toggle and is now deprecated in favour of the organization policy setting. See `status.phase` for the phase the runner is actually in.",
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"variant": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Runner variant (`RUNNER_VARIANT_STANDARD`, `RUNNER_VARIANT_ENTERPRISE`).",
					},
					"configuration": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"auto_update": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether the runner auto-updates.",
							},
							"devcontainer_image_cache_enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Whether the devcontainer build cache is enabled.",
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
									"managed_metrics_enabled": schema.BoolAttribute{
										Optional:            true,
										MarkdownDescription: "When true, the runner pushes metrics to the management plane instead of directly to the remote_write endpoint.",
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
							"update_window": schema.SingleNestedAttribute{
								Optional:            true,
								MarkdownDescription: "Daily time window (UTC) during which auto-updates are allowed. Must be at least 2 hours. Overnight windows supported (e.g. start_hour=22, end_hour=4).",
								Attributes: map[string]schema.Attribute{
									"start_hour": schema.Int64Attribute{
										Required:            true,
										MarkdownDescription: "Start of the update window as a UTC hour (0-23).",
									},
									"end_hour": schema.Int64Attribute{
										Optional:            true,
										Computed:            true,
										MarkdownDescription: "End of the update window as a UTC hour (0-23). Defaults to start_hour + 2.",
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
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
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

	params := &v1.CreateRunnerRequest{
		Name:     plan.Name.ValueString(),
		Provider: enumValue[v1.RunnerProvider]("provider_type", plan.ProviderType.ValueString(), v1.RunnerProvider_value, &resp.Diagnostics),
	}
	if !plan.RunnerManagerID.IsNull() && !plan.RunnerManagerID.IsUnknown() && plan.RunnerManagerID.ValueString() != "" {
		params.RunnerManagerId = plan.RunnerManagerID.ValueString()
	}
	if plan.Spec != nil {
		params.Spec = buildSpecParam(plan.Spec, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Services.Runner.CreateRunner(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create runner", err.Error())
		return
	}

	// Once the runner exists, Create must never return without persisting state,
	// or the remote runner is orphaned. createResp.Runner is the fallback used
	// whenever a follow-up call fails.
	runnerID := createResp.Msg.GetRunner().GetRunnerId()
	runner := createResp.Msg.GetRunner()

	// Read back to populate computed configuration fields (e.g. release_channel,
	// log_level) the create response may omit. If the read fails, fall back to
	// the create response so the runner is still tracked; computed fields
	// reconcile on the next refresh.
	if getResp, getErr := r.client.Services.Runner.GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: runnerID,
	})); getErr == nil {
		runner = getResp.Msg.GetRunner()
	} else {
		resp.Diagnostics.AddWarning("Could not read runner after create", getErr.Error())
	}

	state := mapRunnerToModel(runner, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Services.Runner.GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read runner", err.Error())
		return
	}

	newState := mapRunnerToModel(getResp.Msg.GetRunner(), state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior runnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := buildRunnerUpdateParams(plan, prior, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Runner.UpdateRunner(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update runner", err.Error())
		return
	}

	// Read back to get computed fields
	getResp, err := r.client.Services.Runner.GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: plan.ID.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read runner after update", err.Error())
		return
	}

	state := mapRunnerToModel(getResp.Msg.GetRunner(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	runnerID := state.ID.ValueString()
	deleteParams := &v1.DeleteRunnerRequest{
		RunnerId: runnerID,
	}
	if state.ProviderType.ValueString() != v1.RunnerProvider_RUNNER_PROVIDER_MANAGED.String() {
		deleteParams.Force = true
	}
	_, err := r.client.Services.Runner.DeleteRunner(ctx, connect.NewRequest(deleteParams))
	if err != nil {
		if isAPINotFound(err) {
			return // already gone
		}
		resp.Diagnostics.AddError("Failed to delete runner", err.Error())
		return
	}

	// Poll until the runner reaches DELETED phase or disappears (404).
	if _, err := r.waitForPhase(ctx, runnerID, v1.RunnerPhase_RUNNER_PHASE_DELETED); err != nil {
		resp.Diagnostics.AddError("Runner deletion did not complete", err.Error())
	}
}

// waitForPhase polls the runner status until it reaches the expected phase
// or the API returns 404 (treated as success for deletion). After deleting a
// runner, the API may keep returning the runner in ACTIVE for a short time and
// then start returning 404. In testing, it never returned RUNNER_PHASE_DELETED.
func (r *runnerResource) waitForPhase(ctx context.Context, runnerID string, expected v1.RunnerPhase) (*v1.Runner, error) {
	const (
		pollInterval = 2 * time.Second
		timeout      = 2 * time.Minute
	)

	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for runner %s to reach phase %s", runnerID, expected.String())
		}

		getResp, err := r.client.Services.Runner.GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
			RunnerId: runnerID,
		}))
		if err != nil {
			if isAPINotFound(err) {
				return nil, nil // Delete completion is observed as 404 in practice; RUNNER_PHASE_DELETED is not returned.
			}
			return nil, fmt.Errorf("error polling runner status: %w", err)
		}

		phase := getResp.Msg.GetRunner().GetStatus().GetPhase()
		tflog.Debug(ctx, "waiting for runner phase", map[string]any{
			"runner_id": runnerID,
			"current":   phase.String(),
			"expected":  expected.String(),
		})

		if phase == expected {
			return getResp.Msg.GetRunner(), nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

func (r *runnerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helpers

func buildSpecParam(spec *runnerSpecModel, diagnostics *diag.Diagnostics) *v1.RunnerSpec {
	p := &v1.RunnerSpec{}
	if !spec.Variant.IsNull() && !spec.Variant.IsUnknown() {
		p.Variant = enumValue[v1.RunnerVariant]("spec.variant", spec.Variant.ValueString(), v1.RunnerVariant_value, diagnostics)
	}
	if spec.Configuration != nil {
		p.Configuration = buildConfigParam(spec.Configuration, diagnostics)
	}
	return p
}

func buildConfigParam(cfg *runnerConfigModel, diagnostics *diag.Diagnostics) *v1.RunnerConfiguration {
	p := &v1.RunnerConfiguration{}
	if !cfg.AutoUpdate.IsNull() && !cfg.AutoUpdate.IsUnknown() {
		p.AutoUpdate = cfg.AutoUpdate.ValueBool()
	}
	if !cfg.DevcontainerImageCacheEnabled.IsNull() && !cfg.DevcontainerImageCacheEnabled.IsUnknown() {
		p.DevcontainerImageCacheEnabled = cfg.DevcontainerImageCacheEnabled.ValueBool()
	}
	if !cfg.Region.IsNull() && !cfg.Region.IsUnknown() {
		p.Region = cfg.Region.ValueString()
	}
	if !cfg.ReleaseChannel.IsNull() && !cfg.ReleaseChannel.IsUnknown() {
		p.ReleaseChannel = enumValue[v1.RunnerReleaseChannel]("spec.configuration.release_channel", cfg.ReleaseChannel.ValueString(), v1.RunnerReleaseChannel_value, diagnostics)
	}
	if !cfg.LogLevel.IsNull() && !cfg.LogLevel.IsUnknown() {
		p.LogLevel = enumValue[v1.LogLevel]("spec.configuration.log_level", cfg.LogLevel.ValueString(), v1.LogLevel_value, diagnostics)
	}
	if cfg.Metrics != nil {
		p.Metrics = buildMetricsParam(cfg.Metrics)
	}
	if cfg.UpdateWindow != nil {
		p.UpdateWindow = buildUpdateWindowParam(cfg.UpdateWindow, diagnostics)
	}
	return p
}

func buildUpdateWindowParam(w *runnerUpdateWindowModel, diagnostics *diag.Diagnostics) *v1.UpdateWindow {
	p := &v1.UpdateWindow{}
	if startHour, ok := validatedHour("spec.configuration.update_window.start_hour", w.StartHour, diagnostics); ok {
		p.StartHour = proto.Uint32(uint32(startHour))
	}
	if !w.EndHour.IsNull() && !w.EndHour.IsUnknown() {
		if endHour, ok := validatedHour("spec.configuration.update_window.end_hour", w.EndHour, diagnostics); ok {
			p.EndHour = proto.Uint32(uint32(endHour))
		}
	}
	return p
}

func buildMetricsParam(m *runnerMetricsModel) *v1.MetricsConfiguration {
	p := &v1.MetricsConfiguration{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p.Enabled = m.Enabled.ValueBool()
	}
	if !m.ManagedMetricsEnabled.IsNull() && !m.ManagedMetricsEnabled.IsUnknown() {
		p.ManagedMetricsEnabled = m.ManagedMetricsEnabled.ValueBool()
	}
	if !m.URL.IsNull() && !m.URL.IsUnknown() {
		p.Url = m.URL.ValueString()
	}
	if !m.Username.IsNull() && !m.Username.IsUnknown() {
		p.Username = m.Username.ValueString()
	}
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p.Password = m.Password.ValueString()
	}
	return p
}

func buildUpdateMetricsParam(m *runnerMetricsModel) *v1.UpdateRunnerRequest_MetricsConfiguration {
	p := &v1.UpdateRunnerRequest_MetricsConfiguration{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p.Enabled = m.Enabled.ValueBoolPointer()
	}
	if !m.ManagedMetricsEnabled.IsNull() && !m.ManagedMetricsEnabled.IsUnknown() {
		p.ManagedMetricsEnabled = m.ManagedMetricsEnabled.ValueBoolPointer()
	}
	if !m.URL.IsNull() && !m.URL.IsUnknown() {
		p.Url = m.URL.ValueStringPointer()
	}
	if !m.Username.IsNull() && !m.Username.IsUnknown() {
		p.Username = m.Username.ValueStringPointer()
	}
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p.Password = m.Password.ValueStringPointer()
	}
	return p
}

func buildRunnerUpdateParams(plan, prior runnerModel, diagnostics *diag.Diagnostics) *v1.UpdateRunnerRequest {
	params := &v1.UpdateRunnerRequest{
		RunnerId: plan.ID.ValueString(),
		Name:     plan.Name.ValueStringPointer(),
	}

	if spec, sendSpec := buildRunnerUpdateSpecParam(plan.Spec, prior.Spec, diagnostics); sendSpec {
		params.Spec = spec
	}

	return params
}

// buildRunnerUpdateSpecParam never sets DesiredPhase. UpdateRunner rejects
// desiredPhase with a failed_precondition: the field only ever applied to local
// runners as an organization-wide toggle, and that path is deprecated in favour
// of the organization policy setting. desired_phase is read-only in the schema,
// so there is nothing to send.
func buildRunnerUpdateSpecParam(spec, prior *runnerSpecModel, diagnostics *diag.Diagnostics) (*v1.UpdateRunnerRequest_Spec, bool) {
	p := &v1.UpdateRunnerRequest_Spec{}
	sendSpec := false

	var priorCfg *runnerConfigModel
	if prior != nil {
		priorCfg = prior.Configuration
	}

	if cfg, sendConfig := buildRunnerUpdateConfigParam(specConfiguration(spec), priorCfg, diagnostics); sendConfig {
		p.Configuration = cfg
		sendSpec = true
	}

	return p, sendSpec
}

func buildRunnerUpdateConfigParam(cfg, prior *runnerConfigModel, diagnostics *diag.Diagnostics) (*v1.UpdateRunnerRequest_RunnerConfiguration, bool) {
	p := &v1.UpdateRunnerRequest_RunnerConfiguration{}
	sendConfig := false

	if cfg != nil {
		if !cfg.AutoUpdate.IsNull() && !cfg.AutoUpdate.IsUnknown() {
			p.AutoUpdate = cfg.AutoUpdate.ValueBoolPointer()
			sendConfig = true
		}
		if !cfg.DevcontainerImageCacheEnabled.IsNull() && !cfg.DevcontainerImageCacheEnabled.IsUnknown() {
			p.DevcontainerImageCacheEnabled = cfg.DevcontainerImageCacheEnabled.ValueBoolPointer()
			sendConfig = true
		}
		if !cfg.ReleaseChannel.IsNull() && !cfg.ReleaseChannel.IsUnknown() {
			p.ReleaseChannel = enumValue[v1.RunnerReleaseChannel]("spec.configuration.release_channel", cfg.ReleaseChannel.ValueString(), v1.RunnerReleaseChannel_value, diagnostics).Enum()
			sendConfig = true
		}
		if !cfg.LogLevel.IsNull() && !cfg.LogLevel.IsUnknown() {
			p.LogLevel = enumValue[v1.LogLevel]("spec.configuration.log_level", cfg.LogLevel.ValueString(), v1.LogLevel_value, diagnostics).Enum()
			sendConfig = true
		}
		if cfg.Metrics != nil {
			p.Metrics = buildUpdateMetricsParam(cfg.Metrics)
			sendConfig = true
		}
		if cfg.UpdateWindow != nil {
			p.UpdateWindow = buildUpdateWindowParam(cfg.UpdateWindow, diagnostics)
			sendConfig = true
		}
	}

	if shouldClearRunnerUpdateWindow(cfg, prior) {
		p.UpdateWindow = &v1.UpdateWindow{}
		sendConfig = true
	}

	return p, sendConfig
}

func specConfiguration(spec *runnerSpecModel) *runnerConfigModel {
	if spec == nil {
		return nil
	}

	return spec.Configuration
}

func shouldClearRunnerUpdateWindow(cfg, prior *runnerConfigModel) bool {
	if prior == nil || prior.UpdateWindow == nil {
		return false
	}

	return cfg == nil || cfg.UpdateWindow == nil
}

func mapRunnerToModel(runner *v1.Runner, prior runnerModel) runnerModel {
	m := runnerModel{
		ID:           types.StringValue(runner.GetRunnerId()),
		Name:         types.StringValue(runner.GetName()),
		ProviderType: types.StringValue(enumString(runner.GetProvider())),
	}

	m.RunnerManagerID = stringValueOrNull(runner.GetRunnerManagerId())

	spec := &runnerSpecModel{
		DesiredPhase: types.StringValue(enumString(runner.GetSpec().GetDesiredPhase())),
	}

	m.Spec = spec

	// Map spec — preserve user-set values the API doesn't return
	if prior.Spec != nil {
		spec.Variant = stringValueOrNull(enumString(runner.GetSpec().GetVariant()))

		if prior.Spec.Configuration != nil {
			configuration := runner.GetSpec().GetConfiguration()
			// auto_update: prefer prior state when explicitly set, as the API
			// may ignore the value for certain runner types (e.g. managed).
			autoUpdate := types.BoolValue(configuration.GetAutoUpdate())
			if !prior.Spec.Configuration.AutoUpdate.IsNull() && !prior.Spec.Configuration.AutoUpdate.IsUnknown() {
				autoUpdate = prior.Spec.Configuration.AutoUpdate
			}
			cfg := &runnerConfigModel{
				AutoUpdate:                    autoUpdate,
				DevcontainerImageCacheEnabled: types.BoolValue(configuration.GetDevcontainerImageCacheEnabled()),
				ReleaseChannel:                types.StringValue(enumString(configuration.GetReleaseChannel())),
				LogLevel:                      types.StringValue(enumString(configuration.GetLogLevel())),
			}
			if configuration.GetRegion() != "" {
				cfg.Region = types.StringValue(configuration.GetRegion())
			} else if !prior.Spec.Configuration.Region.IsNull() {
				cfg.Region = prior.Spec.Configuration.Region
			} else {
				cfg.Region = types.StringNull()
			}
			if prior.Spec.Configuration.Metrics != nil {
				cfg.Metrics = &runnerMetricsModel{
					Enabled:               types.BoolValue(configuration.GetMetrics().GetEnabled()),
					ManagedMetricsEnabled: types.BoolValue(configuration.GetMetrics().GetManagedMetricsEnabled()),
					URL:                   stringValueOrNull(configuration.GetMetrics().GetUrl()),
					Username:              stringValueOrNull(configuration.GetMetrics().GetUsername()),
					// Preserve password from prior state — API doesn't return it
					Password: prior.Spec.Configuration.Metrics.Password,
				}
			}
			if startHour, endHour, ok := mapUpdateWindowValues(configuration.GetUpdateWindow()); ok {
				cfg.UpdateWindow = &runnerUpdateWindowModel{
					StartHour: startHour,
					EndHour:   endHour,
				}
			} else if prior.Spec.Configuration.UpdateWindow != nil {
				// API returned no update_window but user had one configured — it was cleared
				cfg.UpdateWindow = nil
			}
			spec.Configuration = cfg
		}
	}

	m.Status = runnerStatusObjectValue(runner.GetStatus())

	return m
}
