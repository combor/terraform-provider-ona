package provider

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretResource{}
	_ resource.ResourceWithImportState = &secretResource{}
)

type secretResource struct {
	client *sdk.Client
}

func NewSecretResource() resource.Resource {
	return &secretResource{}
}

type secretModel struct {
	ID                             types.String `tfsdk:"id"`
	Name                           types.String `tfsdk:"name"`
	Value                          types.String `tfsdk:"value"`
	ProjectID                      types.String `tfsdk:"project_id"`
	EnvironmentVariable            types.Bool   `tfsdk:"environment_variable"`
	FilePath                       types.String `tfsdk:"file_path"`
	ContainerRegistryBasicAuthHost types.String `tfsdk:"container_registry_basic_auth_host"`
	APIOnly                        types.Bool   `tfsdk:"api_only"`
	CreatorID                      types.String `tfsdk:"creator_id"`
	CreatorPrincipal               types.String `tfsdk:"creator_principal"`
	CreatedAt                      types.String `tfsdk:"created_at"`
	UpdatedAt                      types.String `tfsdk:"updated_at"`
}

func (r *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (r *secretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Gitpod secret scoped to a project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Secret ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable secret name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Plaintext value of the secret.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project ID this secret is scoped to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"environment_variable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the secret is injected as an environment variable with the same name.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"file_path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Absolute path where the secret is mounted as a file.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"container_registry_basic_auth_host": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Registry host for docker config basic auth.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"api_only": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the secret is only available via API/CLI and not auto-injected.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"creator_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the subject that created the secret.",
			},
			"creator_principal": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Principal type of the creator.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the secret was created (RFC3339).",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Timestamp when the secret was last updated (RFC3339).",
			},
		},
	}
}

func (r *secretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &v1.CreateSecretRequest{
		Name: plan.Name.ValueString(),
		Scope: &v1.SecretScope{
			Scope: &v1.SecretScope_ProjectId{ProjectId: plan.ProjectID.ValueString()},
		},
		Value: plan.Value.ValueString(),
	}

	if configured := applySecretMount(params, plan); len(configured) > 1 {
		resp.Diagnostics.AddError(
			"Conflicting secret mount",
			fmt.Sprintf("A secret has exactly one mount, but %s were set. Configure only one of environment_variable, file_path, container_registry_basic_auth_host or api_only.",
				strings.Join(configured, ", ")),
		)
		return
	}

	createResp, err := r.client.Services.Secret.CreateSecret(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create secret", err.Error())
		return
	}

	state := mapSecretToModel(createResp.Msg.GetSecret(), plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret, err := r.findSecretByID(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read secret", err.Error())
		return
	}
	if secret == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := mapSecretToModel(secret, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Secret.UpdateSecretValue(ctx, connect.NewRequest(&v1.UpdateSecretValueRequest{
		SecretId: prior.ID.ValueString(),
		Value:    plan.Value.ValueString(),
	}))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update secret value", err.Error())
		return
	}

	// Re-read to get updated timestamps
	secret, err := r.findSecretByID(ctx, plan.ProjectID.ValueString(), prior.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read secret after update", err.Error())
		return
	}
	if secret == nil {
		resp.Diagnostics.AddError("Secret not found after update",
			fmt.Sprintf("Secret %s was not found after updating its value.", prior.ID.ValueString()))
		return
	}

	state := mapSecretToModel(secret, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Services.Secret.DeleteSecret(ctx, connect.NewRequest(&v1.DeleteSecretRequest{
		SecretId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete secret", err.Error())
	}
}

func (r *secretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectID, secretID, err := parseSecretImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), secretID)...)
}

func parseSecretImportID(importID string) (string, string, error) {
	projectID, secretID, ok := strings.Cut(importID, "/")
	if !ok || projectID == "" || secretID == "" {
		return "", "", fmt.Errorf("expected import identifier in the format <project-id>/<secret-id>")
	}

	return projectID, secretID, nil
}

// findSecretByID lists secrets scoped to a project and returns the one matching the given ID.
// Returns nil if the secret is not found.
// It stops at the first match rather than collecting every page, so a later
// page failing cannot fail a lookup that already found its secret.
func (r *secretResource) findSecretByID(ctx context.Context, projectID, secretID string) (*v1.Secret, error) {
	token := ""
	for {
		listResp, err := r.client.Services.Secret.ListSecrets(ctx, connect.NewRequest(&v1.ListSecretsRequest{
			Filter: &v1.ListSecretsRequest_Filter{
				Scope: &v1.SecretScope{
					Scope: &v1.SecretScope_ProjectId{ProjectId: projectID},
				},
			},
			Pagination: &v1.PaginationRequest{PageSize: 100, Token: token},
		}))
		if err != nil {
			return nil, err
		}

		for _, secret := range listResp.Msg.GetSecrets() {
			if secret.GetId() == secretID {
				return secret, nil
			}
		}

		token = listResp.Msg.GetPagination().GetNextToken()
		if token == "" {
			return nil, nil
		}
	}
}

// applySecretMount sets the mount oneof on params from the plan and reports
// which mount attributes actually select a mount. The REST SDK exposed these as
// four independent fields and left it to the server to reject combinations; the
// oneof can only carry one, so the caller rejects anything ambiguous.
//
// A false boolean or an empty string does not select a mount. These attributes
// are optional-and-computed, so after a create the unused ones are read back
// from the API as false; treating those as configured would make a later
// replace look like a conflict.
func applySecretMount(params *v1.CreateSecretRequest, plan secretModel) []string {
	var configured []string

	if mountSelectedBool(plan.EnvironmentVariable) {
		configured = append(configured, "environment_variable")
		params.Mount = &v1.CreateSecretRequest_EnvironmentVariable{EnvironmentVariable: true}
	}
	if mountSelectedString(plan.FilePath) {
		configured = append(configured, "file_path")
		params.Mount = &v1.CreateSecretRequest_FilePath{FilePath: plan.FilePath.ValueString()}
	}
	if mountSelectedString(plan.ContainerRegistryBasicAuthHost) {
		configured = append(configured, "container_registry_basic_auth_host")
		params.Mount = &v1.CreateSecretRequest_ContainerRegistryBasicAuthHost{ContainerRegistryBasicAuthHost: plan.ContainerRegistryBasicAuthHost.ValueString()}
	}
	if mountSelectedBool(plan.APIOnly) {
		configured = append(configured, "api_only")
		params.Mount = &v1.CreateSecretRequest_ApiOnly{ApiOnly: true}
	}

	return configured
}

func mountSelectedBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueBool()
}

func mountSelectedString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

func mapSecretToModel(secret *v1.Secret, prior secretModel) secretModel {
	m := secretModel{
		ID:                             types.StringValue(secret.GetId()),
		Name:                           stringValueOrNull(secret.GetName()),
		ProjectID:                      stringValueOrNull(secret.GetScope().GetProjectId()),
		EnvironmentVariable:            types.BoolValue(secret.GetEnvironmentVariable()),
		FilePath:                       stringValueOrNull(secret.GetFilePath()),
		ContainerRegistryBasicAuthHost: stringValueOrNull(secret.GetContainerRegistryBasicAuthHost()),
		APIOnly:                        types.BoolValue(secret.GetApiOnly()),
		CreatorID:                      stringValueOrNull(secret.GetCreator().GetId()),
		CreatorPrincipal:               stringValueOrNull(enumString(secret.GetCreator().GetPrincipal())),
		CreatedAt:                      timeValueOrNull(secret.GetCreatedAt()),
		UpdatedAt:                      timeValueOrNull(secret.GetUpdatedAt()),
		// Preserve value from prior state — API doesn't return it
		Value: prior.Value,
	}

	return m
}
