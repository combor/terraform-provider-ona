package provider

import (
	"context"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &runnerScmIntegrationResource{}
	_ resource.ResourceWithImportState = &runnerScmIntegrationResource{}
)

type runnerScmIntegrationResource struct {
	client *gitpod.Client
}

func NewRunnerScmIntegrationResource() resource.Resource {
	return &runnerScmIntegrationResource{}
}

type runnerScmIntegrationModel struct {
	ID                         types.String `tfsdk:"id"`
	RunnerID                   types.String `tfsdk:"runner_id"`
	ScmID                      types.String `tfsdk:"scm_id"`
	Host                       types.String `tfsdk:"host"`
	OAuthClientID              types.String `tfsdk:"oauth_client_id"`
	OAuthPlaintextClientSecret types.String `tfsdk:"oauth_plaintext_client_secret"`
	Pat                        types.Bool   `tfsdk:"pat"`
	IssuerURL                  types.String `tfsdk:"issuer_url"`
	VirtualDirectory           types.String `tfsdk:"virtual_directory"`
}

func (r *runnerScmIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_scm_integration"
}

func (r *runnerScmIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an SCM integration on a Gitpod runner.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SCM integration ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"runner_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this integration belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scm_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SCM identifier from the runner's configuration schema (e.g. `github`, `gitlab`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"host": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SCM host (e.g. `github.com`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"oauth_client_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "OAuth app client ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"oauth_plaintext_client_secret": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OAuth app client secret in plaintext. Encrypted by the runner before storage. Not returned by the API.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pat": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether Personal Access Token authentication is enabled.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"issuer_url": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Override authentication provider URL if it differs from the SCM host.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"virtual_directory": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Virtual directory path for Azure DevOps Server (e.g. `/tfs`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *runnerScmIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *runnerScmIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan runnerScmIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := gitpod.RunnerConfigurationScmIntegrationNewParams{
		RunnerID: gitpod.F(plan.RunnerID.ValueString()),
		ScmID:    gitpod.F(plan.ScmID.ValueString()),
		Host:     gitpod.F(plan.Host.ValueString()),
	}

	if !plan.OAuthClientID.IsNull() && !plan.OAuthClientID.IsUnknown() {
		params.OAuthClientID = gitpod.F(plan.OAuthClientID.ValueString())
	}
	if !plan.OAuthPlaintextClientSecret.IsNull() && !plan.OAuthPlaintextClientSecret.IsUnknown() {
		params.OAuthPlaintextClientSecret = gitpod.F(plan.OAuthPlaintextClientSecret.ValueString())
	}
	if !plan.Pat.IsNull() && !plan.Pat.IsUnknown() {
		params.Pat = gitpod.F(plan.Pat.ValueBool())
	}
	if !plan.IssuerURL.IsNull() && !plan.IssuerURL.IsUnknown() {
		params.IssuerURL = gitpod.F(plan.IssuerURL.ValueString())
	}
	if !plan.VirtualDirectory.IsNull() && !plan.VirtualDirectory.IsUnknown() {
		params.VirtualDirectory = gitpod.F(plan.VirtualDirectory.ValueString())
	}

	createResp, err := r.client.Runners.Configurations.ScmIntegrations.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create SCM integration", err.Error())
		return
	}

	// Create only returns ID; read back for full state.
	getResp, err := r.client.Runners.Configurations.ScmIntegrations.Get(ctx, gitpod.RunnerConfigurationScmIntegrationGetParams{
		ID: gitpod.F(createResp.ID),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read SCM integration after create", err.Error())
		return
	}

	state := mapScmIntegrationToModel(getResp.Integration, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *runnerScmIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state runnerScmIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Runners.Configurations.ScmIntegrations.Get(ctx, gitpod.RunnerConfigurationScmIntegrationGetParams{
		ID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read SCM integration", err.Error())
		return
	}

	newState := mapScmIntegrationToModel(getResp.Integration, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *runnerScmIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan runnerScmIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior runnerScmIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := buildRunnerScmIntegrationUpdateParams(plan, prior)

	_, err := r.client.Runners.Configurations.ScmIntegrations.Update(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update SCM integration", err.Error())
		return
	}

	// Update returns empty; read back for updated state.
	getResp, err := r.client.Runners.Configurations.ScmIntegrations.Get(ctx, gitpod.RunnerConfigurationScmIntegrationGetParams{
		ID: gitpod.F(prior.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read SCM integration after update", err.Error())
		return
	}

	state := mapScmIntegrationToModel(getResp.Integration, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildRunnerScmIntegrationUpdateParams(plan, prior runnerScmIntegrationModel) gitpod.RunnerConfigurationScmIntegrationUpdateParams {
	params := gitpod.RunnerConfigurationScmIntegrationUpdateParams{
		ID: gitpod.F(prior.ID.ValueString()),
	}

	if !plan.OAuthClientID.IsNull() && !plan.OAuthClientID.IsUnknown() {
		params.OAuthClientID = gitpod.F(plan.OAuthClientID.ValueString())
	} else if plan.OAuthClientID.IsNull() && !prior.OAuthClientID.IsNull() && !prior.OAuthClientID.IsUnknown() {
		// Send empty string to clear OAuth config per SDK docs.
		params.OAuthClientID = gitpod.F("")
	}
	if !plan.OAuthPlaintextClientSecret.IsNull() && !plan.OAuthPlaintextClientSecret.IsUnknown() {
		params.OAuthPlaintextClientSecret = gitpod.F(plan.OAuthPlaintextClientSecret.ValueString())
	} else if plan.OAuthPlaintextClientSecret.IsNull() && !prior.OAuthPlaintextClientSecret.IsNull() && !prior.OAuthPlaintextClientSecret.IsUnknown() {
		params.OAuthPlaintextClientSecret = gitpod.F("")
	}
	if !plan.Pat.IsNull() && !plan.Pat.IsUnknown() {
		params.Pat = gitpod.F(plan.Pat.ValueBool())
	}
	if !plan.IssuerURL.IsNull() && !plan.IssuerURL.IsUnknown() {
		params.IssuerURL = gitpod.F(plan.IssuerURL.ValueString())
	}
	if !plan.VirtualDirectory.IsNull() && !plan.VirtualDirectory.IsUnknown() {
		params.VirtualDirectory = gitpod.F(plan.VirtualDirectory.ValueString())
	}

	return params
}

func (r *runnerScmIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state runnerScmIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Runners.Configurations.ScmIntegrations.Delete(ctx, gitpod.RunnerConfigurationScmIntegrationDeleteParams{
		ID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete SCM integration", err.Error())
	}
}

func (r *runnerScmIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func mapScmIntegrationToModel(integration gitpod.ScmIntegration, prior runnerScmIntegrationModel) runnerScmIntegrationModel {
	return runnerScmIntegrationModel{
		ID:               types.StringValue(integration.ID),
		RunnerID:         stringValueOrNull(integration.RunnerID),
		ScmID:            stringValueOrNull(integration.ScmID),
		Host:             stringValueOrNull(integration.Host),
		OAuthClientID:    stringValueOrPriorExplicitEmpty(integration.OAuth.ClientID, prior.OAuthClientID),
		Pat:              types.BoolValue(integration.Pat),
		IssuerURL:        stringValueOrNull(integration.OAuth.IssuerURL),
		VirtualDirectory: stringValueOrNull(integration.VirtualDirectory),
		// Preserve from prior state — API doesn't return plaintext secret
		OAuthPlaintextClientSecret: prior.OAuthPlaintextClientSecret,
	}
}
