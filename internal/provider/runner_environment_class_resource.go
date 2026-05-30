package provider

import (
	"context"
	"fmt"
	"strconv"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &runnerEnvironmentClassResource{}
	_ resource.ResourceWithImportState = &runnerEnvironmentClassResource{}
)

type runnerEnvironmentClassResource struct {
	client *gitpod.Client
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
		MarkdownDescription: `Manages an environment class on a Gitpod runner.

**Supported Runner Types**: This resource supports AWS EC2 (RUNNER_PROVIDER_AWS_EC2) and Managed (RUNNER_PROVIDER_MANAGED) runners only.

**Note**: Environment classes cannot be deleted via the API. Instead, they are disabled to prevent their use.`,
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
				MarkdownDescription: "Environment class configuration for AWS EC2 and Managed runners.",
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

	params := gitpod.RunnerConfigurationEnvironmentClassNewParams{
		RunnerID:      gitpod.F(plan.RunnerID.ValueString()),
		DisplayName:   gitpod.F(plan.DisplayName.ValueString()),
		Configuration: gitpod.F(buildConfigurationFieldValues(plan.Configuration)),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		params.Description = gitpod.F(plan.Description.ValueString())
	}

	createResp, err := r.client.Runners.Configurations.EnvironmentClasses.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment class", err.Error())
		return
	}

	// The API does not support setting enabled during creation.
	// If the user specified enabled=false, update it after creation.
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && !plan.Enabled.ValueBool() {
		updateParams := gitpod.RunnerConfigurationEnvironmentClassUpdateParams{
			EnvironmentClassID: gitpod.F(createResp.ID),
			Enabled:            gitpod.F(false),
		}
		_, err = r.client.Runners.Configurations.EnvironmentClasses.Update(ctx, updateParams)
		if err != nil {
			resp.Diagnostics.AddError("Failed to set enabled state after create", err.Error())
			return
		}
	}

	// Create only returns ID; read back for full state.
	getResp, err := r.client.Runners.Configurations.EnvironmentClasses.Get(ctx, gitpod.RunnerConfigurationEnvironmentClassGetParams{
		EnvironmentClassID: gitpod.F(createResp.ID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment class after create", err.Error())
		return
	}

	state := mapEnvironmentClassToModel(getResp.EnvironmentClass)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerEnvironmentClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerEnvironmentClassModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Runners.Configurations.EnvironmentClasses.Get(ctx, gitpod.RunnerConfigurationEnvironmentClassGetParams{
		EnvironmentClassID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read environment class", err.Error())
		return
	}

	newState := mapEnvironmentClassToModel(getResp.EnvironmentClass)
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

	params := gitpod.RunnerConfigurationEnvironmentClassUpdateParams{
		EnvironmentClassID: gitpod.F(prior.ID.ValueString()),
		DisplayName:        gitpod.F(plan.DisplayName.ValueString()),
	}

	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		params.Description = gitpod.F(plan.Description.ValueString())
	}
	if !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() {
		params.Enabled = gitpod.F(plan.Enabled.ValueBool())
	}

	_, err := r.client.Runners.Configurations.EnvironmentClasses.Update(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update environment class", err.Error())
		return
	}

	// Update returns empty; read back for updated state.
	getResp, err := r.client.Runners.Configurations.EnvironmentClasses.Get(ctx, gitpod.RunnerConfigurationEnvironmentClassGetParams{
		EnvironmentClassID: gitpod.F(prior.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment class after update", err.Error())
		return
	}

	state := mapEnvironmentClassToModel(getResp.EnvironmentClass)
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
	_, err := r.client.Runners.Configurations.EnvironmentClasses.Update(ctx, gitpod.RunnerConfigurationEnvironmentClassUpdateParams{
		EnvironmentClassID: gitpod.F(state.ID.ValueString()),
		Enabled:            gitpod.F(false),
	})
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

func buildConfigurationFieldValues(cfg *runnerEnvironmentClassConfigurationModel) []shared.FieldValueParam {
	if cfg == nil {
		return nil
	}

	fields := make([]shared.FieldValueParam, 0, 3)

	// instanceType is required
	if !cfg.InstanceType.IsNull() && !cfg.InstanceType.IsUnknown() {
		fields = append(fields, shared.FieldValueParam{
			Key:   gitpod.F("instanceType"),
			Value: gitpod.F(cfg.InstanceType.ValueString()),
		})
	}

	// diskSizeGB is optional
	if !cfg.DiskSizeGB.IsNull() && !cfg.DiskSizeGB.IsUnknown() {
		fields = append(fields, shared.FieldValueParam{
			Key:   gitpod.F("diskSizeGB"),
			Value: gitpod.F(fmt.Sprintf("%d", cfg.DiskSizeGB.ValueInt64())),
		})
	}

	// spot is optional
	if !cfg.Spot.IsNull() && !cfg.Spot.IsUnknown() {
		fields = append(fields, shared.FieldValueParam{
			Key:   gitpod.F("spot"),
			Value: gitpod.F(fmt.Sprintf("%t", cfg.Spot.ValueBool())),
		})
	}

	return fields
}

func mapConfigurationToModel(fields []shared.FieldValue) *runnerEnvironmentClassConfigurationModel {
	if len(fields) == 0 {
		return nil
	}

	cfg := &runnerEnvironmentClassConfigurationModel{}

	for _, field := range fields {
		switch field.Key {
		case "instanceType":
			cfg.InstanceType = types.StringValue(field.Value)
		case "diskSizeGB":
			if val, err := strconv.ParseInt(field.Value, 10, 64); err == nil {
				cfg.DiskSizeGB = types.Int64Value(val)
			} else {
				cfg.DiskSizeGB = types.Int64Null()
			}
		case "spot":
			if val, err := strconv.ParseBool(field.Value); err == nil {
				cfg.Spot = types.BoolValue(val)
			} else {
				cfg.Spot = types.BoolNull()
			}
		}
	}

	return cfg
}

func mapEnvironmentClassToModel(environmentClass gitpod.EnvironmentClass) runnerEnvironmentClassModel {
	return runnerEnvironmentClassModel{
		ID:            types.StringValue(environmentClass.ID),
		RunnerID:      types.StringValue(environmentClass.RunnerID),
		DisplayName:   stringValueOrNull(environmentClass.DisplayName),
		Description:   stringValueOrNull(environmentClass.Description),
		Configuration: mapConfigurationToModel(environmentClass.Configuration),
		Enabled:       types.BoolValue(environmentClass.Enabled),
	}
}
