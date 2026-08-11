package provider

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/proto"
)

var (
	_ resource.Resource                = &runnerEnvironmentClassResource{}
	_ resource.ResourceWithImportState = &runnerEnvironmentClassResource{}
)

type runnerEnvironmentClassResource struct {
	client *sdk.Client
}

func NewRunnerEnvironmentClassResource() resource.Resource {
	return &runnerEnvironmentClassResource{}
}

type runnerEnvironmentClassModel struct {
	ID            types.String                              `tfsdk:"id"`
	RunnerID      types.String                              `tfsdk:"runner_id"`
	DisplayName   types.String                              `tfsdk:"display_name"`
	Description   types.String                              `tfsdk:"description"`
	Configuration *runnerEnvironmentClassConfigurationModel `tfsdk:"configuration"`
	Enabled       types.Bool                                `tfsdk:"enabled"`
}

type runnerEnvironmentClassConfigurationModel struct {
	InstanceType types.String `tfsdk:"instance_type"`
	DiskSizeGB   types.Int64  `tfsdk:"disk_size_gb"`
	Spot         types.Bool   `tfsdk:"spot"`
}

func (r *runnerEnvironmentClassResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_environment_class"
}

func (r *runnerEnvironmentClassResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages an environment class on a self-hosted Gitpod runner (e.g. AWS EC2).

**Supported Runner Types**: Self-hosted runners only. Managed (RUNNER_PROVIDER_MANAGED) runners do not support custom environment classes; the API rejects CreateEnvironmentClass for them with a 403.

**Note**: Environment classes cannot be deleted via the API; on destroy they are disabled instead. The configuration fields are immutable, so changing any of them replaces the class and leaves the previous one behind (disabled).`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Environment class ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"runner_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this environment class belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable environment class name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Human-readable environment class description.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"configuration": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Environment class configuration for AWS EC2 runners.",
				Attributes: map[string]schema.Attribute{
					"instance_type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "AWS EC2 instance type (e.g., 'm6i.large', 't3.medium').",
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"disk_size_gb": schema.Int64Attribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Disk size in GB.",
						PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown(), int64planmodifier.RequiresReplace()},
					},
					"spot": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Use spot instances.",
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown(), boolplanmodifier.RequiresReplace()},
					},
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the environment class can be used to create new environments.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *runnerEnvironmentClassResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *runnerEnvironmentClassResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &v1.CreateEnvironmentClassRequest{
		RunnerId:      plan.RunnerID.ValueString(),
		DisplayName:   plan.DisplayName.ValueString(),
		Configuration: buildConfigurationFieldValues(plan.Configuration),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		params.Description = plan.Description.ValueString()
	}

	createResp, err := r.client.Services.RunnerConfiguration.CreateEnvironmentClass(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment class", err.Error())
		return
	}

	// The class now exists. The API has no delete (Delete only disables), so
	// Create must never return without persisting state or the class is
	// permanently orphaned. fallback carries the ID if a follow-up call fails.
	id := createResp.Msg.GetId()
	fallback := envClassPlanWithID(plan, id)

	// The API ignores enabled during creation; apply the configured value with a
	// follow-up Update whenever it is set (the create default is not relied on).
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		if _, err = r.client.Services.RunnerConfiguration.UpdateEnvironmentClass(ctx, connect.NewRequest(&v1.UpdateEnvironmentClassRequest{
			EnvironmentClassId: id,
			Enabled:            plan.Enabled.ValueBoolPointer(),
		})); err != nil {
			resp.Diagnostics.Append(resp.State.Set(ctx, &fallback)...)
			resp.Diagnostics.AddError("Failed to set enabled state after create", err.Error())
			return
		}
	}

	// Create only returns the ID; read back for full state. If the read fails,
	// fall back to the plan so the class is tracked; the next refresh reconciles
	// computed fields.
	if getResp, getErr := r.client.Services.RunnerConfiguration.GetEnvironmentClass(ctx, connect.NewRequest(&v1.GetEnvironmentClassRequest{
		EnvironmentClassId: id,
	})); getErr == nil {
		state := mapEnvironmentClassToModel(getResp.Msg.GetEnvironmentClass())
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	} else {
		resp.Diagnostics.Append(resp.State.Set(ctx, &fallback)...)
		resp.Diagnostics.AddWarning("Could not read environment class after create", getErr.Error())
	}
}

func (r *runnerEnvironmentClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Services.RunnerConfiguration.GetEnvironmentClass(ctx, connect.NewRequest(&v1.GetEnvironmentClassRequest{
		EnvironmentClassId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read environment class", err.Error())
		return
	}

	newState := mapEnvironmentClassToModel(getResp.Msg.GetEnvironmentClass())
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerEnvironmentClassResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &v1.UpdateEnvironmentClassRequest{
		EnvironmentClassId: prior.ID.ValueString(),
		DisplayName:        plan.DisplayName.ValueStringPointer(),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		params.Description = plan.Description.ValueStringPointer()
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		params.Enabled = plan.Enabled.ValueBoolPointer()
	}

	_, err := r.client.Services.RunnerConfiguration.UpdateEnvironmentClass(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update environment class", err.Error())
		return
	}

	// Update returns empty; read back for updated state.
	getResp, err := r.client.Services.RunnerConfiguration.GetEnvironmentClass(ctx, connect.NewRequest(&v1.GetEnvironmentClassRequest{
		EnvironmentClassId: prior.ID.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment class after update", err.Error())
		return
	}

	state := mapEnvironmentClassToModel(getResp.Msg.GetEnvironmentClass())
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerEnvironmentClassResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API does not support deleting environment classes.
	// Instead, disable the environment class to prevent it from being used.
	_, err := r.client.Services.RunnerConfiguration.UpdateEnvironmentClass(ctx, connect.NewRequest(&v1.UpdateEnvironmentClassRequest{
		EnvironmentClassId: state.ID.ValueString(),
		Enabled:            proto.Bool(false),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to disable environment class", err.Error())
	}
}

func (r *runnerEnvironmentClassResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// envClassPlanWithID returns a best-effort model from the plan plus the created
// ID, resolving computed-and-unknown values to null so it is valid to persist.
// It keeps a created class tracked when a follow-up call fails (the API has no
// delete, so an unpersisted class is orphaned permanently).
func envClassPlanWithID(plan runnerEnvironmentClassModel, id string) runnerEnvironmentClassModel {
	m := plan
	m.ID = types.StringValue(id)
	if m.Description.IsUnknown() {
		m.Description = types.StringNull()
	}
	if m.Enabled.IsUnknown() {
		m.Enabled = types.BoolNull()
	}
	if plan.Configuration != nil {
		cfg := *plan.Configuration
		if cfg.DiskSizeGB.IsUnknown() {
			cfg.DiskSizeGB = types.Int64Null()
		}
		if cfg.Spot.IsUnknown() {
			cfg.Spot = types.BoolNull()
		}
		m.Configuration = &cfg
	}
	return m
}

func buildConfigurationFieldValues(cfg *runnerEnvironmentClassConfigurationModel) []*v1.FieldValue {
	if cfg == nil {
		return nil
	}

	fields := make([]*v1.FieldValue, 0, 3)

	// instanceType is required
	if !cfg.InstanceType.IsNull() && !cfg.InstanceType.IsUnknown() {
		fields = append(fields, &v1.FieldValue{
			Key:   "instanceType",
			Value: cfg.InstanceType.ValueString(),
		})
	}

	// diskSizeGB is optional
	if !cfg.DiskSizeGB.IsNull() && !cfg.DiskSizeGB.IsUnknown() {
		fields = append(fields, &v1.FieldValue{
			Key:   "diskSizeGB",
			Value: fmt.Sprintf("%d", cfg.DiskSizeGB.ValueInt64()),
		})
	}

	// spot is optional
	if !cfg.Spot.IsNull() && !cfg.Spot.IsUnknown() {
		fields = append(fields, &v1.FieldValue{
			Key:   "spot",
			Value: fmt.Sprintf("%t", cfg.Spot.ValueBool()),
		})
	}

	return fields
}

func mapConfigurationToModel(fields []*v1.FieldValue) *runnerEnvironmentClassConfigurationModel {
	if len(fields) == 0 {
		return nil
	}

	cfg := &runnerEnvironmentClassConfigurationModel{}

	for _, field := range fields {
		switch field.GetKey() {
		case "instanceType":
			cfg.InstanceType = types.StringValue(field.GetValue())
		case "diskSizeGB":
			if val, err := strconv.ParseInt(field.GetValue(), 10, 64); err == nil {
				cfg.DiskSizeGB = types.Int64Value(val)
			} else {
				cfg.DiskSizeGB = types.Int64Null()
			}
		case "spot":
			if val, err := strconv.ParseBool(field.GetValue()); err == nil {
				cfg.Spot = types.BoolValue(val)
			} else {
				cfg.Spot = types.BoolNull()
			}
		}
	}

	return cfg
}

func mapEnvironmentClassToModel(environmentClass *v1.EnvironmentClass) runnerEnvironmentClassModel {
	return runnerEnvironmentClassModel{
		ID:            types.StringValue(environmentClass.GetId()),
		RunnerID:      types.StringValue(environmentClass.GetRunnerId()),
		DisplayName:   stringValueOrNull(environmentClass.GetDisplayName()),
		Description:   stringValueOrNull(environmentClass.GetDescription()),
		Configuration: mapConfigurationToModel(environmentClass.GetConfiguration()),
		Enabled:       types.BoolValue(environmentClass.GetEnabled()),
	}
}
