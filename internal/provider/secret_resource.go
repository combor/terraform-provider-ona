package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	_ resource.Resource                = &secretResource{}
	_ resource.ResourceWithImportState = &secretResource{}
)

type secretResource struct {
	client *gitpod.Client
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

func (r *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := gitpod.SecretNewParams{
		Name: gitpod.F(plan.Name.ValueString()),
		Scope: gitpod.F(gitpod.SecretScopeParam{
			ProjectID: gitpod.F(plan.ProjectID.ValueString()),
		}),
		Value: gitpod.F(plan.Value.ValueString()),
	}

	if !plan.EnvironmentVariable.IsNull() && !plan.EnvironmentVariable.IsUnknown() {
		params.EnvironmentVariable = gitpod.F(plan.EnvironmentVariable.ValueBool())
	}
	if !plan.FilePath.IsNull() && !plan.FilePath.IsUnknown() {
		params.FilePath = gitpod.F(plan.FilePath.ValueString())
	}
	if !plan.ContainerRegistryBasicAuthHost.IsNull() && !plan.ContainerRegistryBasicAuthHost.IsUnknown() {
		params.ContainerRegistryBasicAuthHost = gitpod.F(plan.ContainerRegistryBasicAuthHost.ValueString())
	}
	if !plan.APIOnly.IsNull() && !plan.APIOnly.IsUnknown() {
		params.APIOnly = gitpod.F(plan.APIOnly.ValueBool())
	}

	createResp, err := r.client.Secrets.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create secret", err.Error())
		return
	}

	state := mapSecretToModel(createResp.Secret, plan)
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
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
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

	newState := mapSecretToModel(*secret, state)
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

	_, err := r.client.Secrets.UpdateValue(ctx, gitpod.SecretUpdateValueParams{
		SecretID: gitpod.F(prior.ID.ValueString()),
		Value:    gitpod.F(plan.Value.ValueString()),
	})
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

	state := mapSecretToModel(*secret, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Secrets.Delete(ctx, gitpod.SecretDeleteParams{
		SecretID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
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
func (r *secretResource) findSecretByID(ctx context.Context, projectID, secretID string) (*gitpod.Secret, error) {
	iter := r.client.Secrets.ListAutoPaging(ctx, gitpod.SecretListParams{
		Filter: gitpod.F(gitpod.SecretListParamsFilter{
			Scope: gitpod.F(gitpod.SecretScopeParam{
				ProjectID: gitpod.F(projectID),
			}),
		}),
	})

	for iter.Next() {
		secret := iter.Current()
		if secret.ID == secretID {
			return &secret, nil
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return nil, nil
}

func mapSecretToModel(secret gitpod.Secret, prior secretModel) secretModel {
	m := secretModel{
		ID:                             types.StringValue(secret.ID),
		Name:                           stringValueOrNull(secret.Name),
		ProjectID:                      stringValueOrNull(secret.Scope.ProjectID),
		EnvironmentVariable:            types.BoolValue(secret.EnvironmentVariable),
		FilePath:                       stringValueOrNull(secret.FilePath),
		ContainerRegistryBasicAuthHost: stringValueOrNull(secret.ContainerRegistryBasicAuthHost),
		APIOnly:                        types.BoolValue(secret.APIOnly),
		CreatorID:                      stringValueOrNull(secret.Creator.ID),
		CreatorPrincipal:               stringValueOrNull(string(secret.Creator.Principal)),
		CreatedAt:                      timeValueOrNull(secret.CreatedAt),
		UpdatedAt:                      timeValueOrNull(secret.UpdatedAt),
		// Preserve value from prior state — API doesn't return it
		Value: prior.Value,
	}

	return m
}
