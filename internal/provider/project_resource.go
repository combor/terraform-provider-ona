package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/shared"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResource struct {
	client *gitpod.Client
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

type projectModel struct {
	ID                    types.String             `tfsdk:"id"`
	Name                  types.String             `tfsdk:"name"`
	AutomationsFilePath   types.String             `tfsdk:"automations_file_path"`
	DevcontainerFilePath  types.String             `tfsdk:"devcontainer_file_path"`
	Initializer           *projectInitializerModel `tfsdk:"initializer"`
	PrebuildConfiguration types.Object             `tfsdk:"prebuild_configuration"`
	RecommendedEditors    types.Map                `tfsdk:"recommended_editors"`
	TechnicalDescription  types.String             `tfsdk:"technical_description"`
	DesiredPhase          types.String             `tfsdk:"desired_phase"`
	Metadata              types.Object             `tfsdk:"metadata"`
	UsedBy                types.Object             `tfsdk:"used_by"`
}

type projectInitializerModel struct {
	Specs []projectInitializerSpecModel `tfsdk:"specs"`
}

type projectInitializerSpecModel struct {
	ContextURL *projectInitializerContextURLModel `tfsdk:"context_url"`
	Git        *projectInitializerGitModel        `tfsdk:"git"`
}

type projectInitializerContextURLModel struct {
	URL types.String `tfsdk:"url"`
}

type projectInitializerGitModel struct {
	CheckoutLocation  types.String `tfsdk:"checkout_location"`
	CloneTarget       types.String `tfsdk:"clone_target"`
	RemoteURI         types.String `tfsdk:"remote_uri"`
	TargetMode        types.String `tfsdk:"target_mode"`
	UpstreamRemoteURI types.String `tfsdk:"upstream_remote_uri"`
}

type projectPrebuildConfigurationModel struct {
	Enabled               types.Bool                   `tfsdk:"enabled"`
	EnableJetbrainsWarmup types.Bool                   `tfsdk:"enable_jetbrains_warmup"`
	EnvironmentClassIDs   types.List                   `tfsdk:"environment_class_ids"`
	Executor              *projectSubjectModel         `tfsdk:"executor"`
	Timeout               types.String                 `tfsdk:"timeout"`
	Trigger               *projectPrebuildTriggerModel `tfsdk:"trigger"`
}

type projectPrebuildTriggerModel struct {
	DailySchedule *projectPrebuildDailyScheduleModel `tfsdk:"daily_schedule"`
}

type projectPrebuildDailyScheduleModel struct {
	HourUTC types.Int64 `tfsdk:"hour_utc"`
}

type projectRecommendedEditor struct {
	Versions types.List `tfsdk:"versions"`
}

type projectMetadataModel struct {
	Name           types.String         `tfsdk:"name"`
	OrganizationID types.String         `tfsdk:"organization_id"`
	CreatedAt      types.String         `tfsdk:"created_at"`
	UpdatedAt      types.String         `tfsdk:"updated_at"`
	Creator        *projectSubjectModel `tfsdk:"creator"`
}

type projectUsedByModel struct {
	TotalSubjects types.Int64           `tfsdk:"total_subjects"`
	Subjects      []projectSubjectModel `tfsdk:"subjects"`
}

type projectSubjectModel struct {
	ID        types.String `tfsdk:"id"`
	Principal types.String `tfsdk:"principal"`
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Gitpod project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project ID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable project name.",
			},
			"automations_file_path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Path to the automations file relative to the repository root.",
			},
			"devcontainer_file_path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Path to the devcontainer file relative to the repository root.",
			},
			"initializer": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Defines how the project content is initialized.",
				Attributes: map[string]schema.Attribute{
					"specs": schema.ListNestedAttribute{
						Required:            true,
						MarkdownDescription: "Initializer specs. Each entry may define `context_url`, `git`, or both.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"context_url": schema.SingleNestedAttribute{
									Optional:            true,
									MarkdownDescription: "URL used to initialize the project context.",
									Attributes: map[string]schema.Attribute{
										"url": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Source URL for the context.",
										},
									},
								},
								"git": schema.SingleNestedAttribute{
									Optional:            true,
									MarkdownDescription: "Git repository initializer settings.",
									Attributes: map[string]schema.Attribute{
										"checkout_location": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Relative checkout path inside the environment.",
										},
										"clone_target": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Clone target interpreted according to `target_mode`.",
										},
										"remote_uri": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Git remote URI.",
										},
										"target_mode": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Git clone target mode.",
										},
										"upstream_remote_uri": schema.StringAttribute{
											Optional:            true,
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
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Prebuild configuration for the project.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Whether prebuilds are enabled.",
					},
					"enable_jetbrains_warmup": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Whether JetBrains warmup runs during prebuilds.",
					},
					"environment_class_ids": schema.ListAttribute{
						ElementType:         types.StringType,
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Environment class IDs that should receive prebuilds.",
					},
					"executor": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Subject whose SCM credentials are used for prebuilds.",
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Executor subject ID.",
							},
							"principal": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "Executor principal.",
							},
						},
					},
					"timeout": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Maximum prebuild duration, such as `3600s`.",
					},
					"trigger": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Prebuild trigger settings.",
						Attributes: map[string]schema.Attribute{
							"daily_schedule": schema.SingleNestedAttribute{
								Required:            true,
								MarkdownDescription: "Daily schedule trigger.",
								Attributes: map[string]schema.Attribute{
									"hour_utc": schema.Int64Attribute{
										Required:            true,
										MarkdownDescription: "UTC hour (0-23) for the daily prebuild trigger.",
									},
								},
							},
						},
					},
				},
			},
			"recommended_editors": schema.MapNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Recommended editors keyed by editor alias.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"versions": schema.ListAttribute{
							ElementType:         types.StringType,
							Required:            true,
							MarkdownDescription: "Recommended versions. Use an empty list to recommend all available versions.",
						},
					},
				},
			},
			"technical_description": schema.StringAttribute{
				Optional:            true,
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

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := buildProjectNewParams(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.client.Projects.New(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project", err.Error())
		return
	}

	project := createResp.Project

	recommendedEditorsModel, recommendedEditorsDiags := projectRecommendedEditorsFromMap(ctx, plan.RecommendedEditors)
	resp.Diagnostics.Append(recommendedEditorsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if recommendedEditorsModel != nil {
		updateParams := gitpod.ProjectUpdateParams{
			ProjectID: gitpod.F(project.ID),
		}
		recommendedEditors, updateDiags := buildRecommendedEditorsParam(ctx, recommendedEditorsModel)
		resp.Diagnostics.Append(updateDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateParams.RecommendedEditors = gitpod.F(recommendedEditors)

		updateResp, err := r.client.Projects.Update(ctx, updateParams)
		if err != nil {
			resp.Diagnostics.AddError("Failed to set recommended editors", err.Error())
			return
		}

		project = updateResp.Project
	}

	state, diags := mapProjectToModel(ctx, project, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getResp, err := r.client.Projects.Get(ctx, gitpod.ProjectGetParams{
		ProjectID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read project", err.Error())
		return
	}

	newState, diags := mapProjectToModel(ctx, getResp.Project, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params, diags := buildProjectUpdateParams(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateResp, err := r.client.Projects.Update(ctx, params)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update project", err.Error())
		return
	}

	state, diags := mapProjectToModel(ctx, updateResp.Project, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Projects.Delete(ctx, gitpod.ProjectDeleteParams{
		ProjectID: gitpod.F(state.ID.ValueString()),
	})
	if err != nil {
		var apiErr *gitpod.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return
		}

		resp.Diagnostics.AddError("Failed to delete project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildProjectNewParams(ctx context.Context, plan projectModel) (gitpod.ProjectNewParams, diag.Diagnostics) {
	var diags diag.Diagnostics

	initializer, initDiags := buildEnvironmentInitializerParam(plan.Initializer)
	diags.Append(initDiags...)
	if diags.HasError() {
		return gitpod.ProjectNewParams{}, diags
	}

	params := gitpod.ProjectNewParams{
		Name:        gitpod.F(plan.Name.ValueString()),
		Initializer: gitpod.F(initializer),
	}

	if !plan.AutomationsFilePath.IsNull() && !plan.AutomationsFilePath.IsUnknown() {
		params.AutomationsFilePath = gitpod.F(plan.AutomationsFilePath.ValueString())
	}
	if !plan.DevcontainerFilePath.IsNull() && !plan.DevcontainerFilePath.IsUnknown() {
		params.DevcontainerFilePath = gitpod.F(plan.DevcontainerFilePath.ValueString())
	}
	if !plan.TechnicalDescription.IsNull() && !plan.TechnicalDescription.IsUnknown() {
		params.TechnicalDescription = gitpod.F(plan.TechnicalDescription.ValueString())
	}
	prebuildModel, prebuildModelDiags := projectPrebuildConfigurationModelFromObject(ctx, plan.PrebuildConfiguration)
	diags.Append(prebuildModelDiags...)
	if diags.HasError() {
		return gitpod.ProjectNewParams{}, diags
	}
	if prebuildModel != nil {
		prebuild, prebuildDiags := buildProjectPrebuildConfigurationParam(ctx, prebuildModel)
		diags.Append(prebuildDiags...)
		if diags.HasError() {
			return gitpod.ProjectNewParams{}, diags
		}
		params.PrebuildConfiguration = gitpod.F(prebuild)
	}

	return params, diags
}

func buildProjectUpdateParams(ctx context.Context, plan projectModel) (gitpod.ProjectUpdateParams, diag.Diagnostics) {
	var diags diag.Diagnostics

	params := gitpod.ProjectUpdateParams{
		ProjectID: gitpod.F(plan.ID.ValueString()),
		Name:      gitpod.F(plan.Name.ValueString()),
	}

	initializer, initDiags := buildEnvironmentInitializerParam(plan.Initializer)
	diags.Append(initDiags...)
	if diags.HasError() {
		return gitpod.ProjectUpdateParams{}, diags
	}
	params.Initializer = gitpod.F(initializer)

	if !plan.AutomationsFilePath.IsNull() && !plan.AutomationsFilePath.IsUnknown() {
		params.AutomationsFilePath = gitpod.F(plan.AutomationsFilePath.ValueString())
	}
	if !plan.DevcontainerFilePath.IsNull() && !plan.DevcontainerFilePath.IsUnknown() {
		params.DevcontainerFilePath = gitpod.F(plan.DevcontainerFilePath.ValueString())
	}
	if !plan.TechnicalDescription.IsNull() && !plan.TechnicalDescription.IsUnknown() {
		params.TechnicalDescription = gitpod.F(plan.TechnicalDescription.ValueString())
	}
	prebuildModel, prebuildModelDiags := projectPrebuildConfigurationModelFromObject(ctx, plan.PrebuildConfiguration)
	diags.Append(prebuildModelDiags...)
	if diags.HasError() {
		return gitpod.ProjectUpdateParams{}, diags
	}
	if prebuildModel != nil {
		prebuild, prebuildDiags := buildProjectPrebuildConfigurationParam(ctx, prebuildModel)
		diags.Append(prebuildDiags...)
		if diags.HasError() {
			return gitpod.ProjectUpdateParams{}, diags
		}
		params.PrebuildConfiguration = gitpod.F(prebuild)
	}
	recommendedEditorsModel, recommendedEditorsDiags := projectRecommendedEditorsFromMap(ctx, plan.RecommendedEditors)
	diags.Append(recommendedEditorsDiags...)
	if diags.HasError() {
		return gitpod.ProjectUpdateParams{}, diags
	}
	if recommendedEditorsModel != nil {
		recommendedEditors, recDiags := buildRecommendedEditorsParam(ctx, recommendedEditorsModel)
		diags.Append(recDiags...)
		if diags.HasError() {
			return gitpod.ProjectUpdateParams{}, diags
		}
		params.RecommendedEditors = gitpod.F(recommendedEditors)
	}

	return params, diags
}

func buildEnvironmentInitializerParam(initializer *projectInitializerModel) (gitpod.EnvironmentInitializerParam, diag.Diagnostics) {
	var diags diag.Diagnostics

	if initializer == nil || len(initializer.Specs) == 0 {
		diags.AddError("Missing initializer specs", "initializer.specs must contain at least one entry.")
		return gitpod.EnvironmentInitializerParam{}, diags
	}

	specs := make([]gitpod.EnvironmentInitializerSpecParam, 0, len(initializer.Specs))
	for idx, spec := range initializer.Specs {
		if spec.ContextURL == nil && spec.Git == nil {
			diags.AddError("Invalid initializer spec",
				fmt.Sprintf("initializer.specs[%d] must set at least one of context_url or git.", idx))
			continue
		}

		specParam := gitpod.EnvironmentInitializerSpecParam{}
		if spec.ContextURL != nil {
			specParam.ContextURL = gitpod.F(gitpod.EnvironmentInitializerSpecsContextURLParam{
				URL: gitpod.F(spec.ContextURL.URL.ValueString()),
			})
		}
		if spec.Git != nil {
			gitParam := gitpod.EnvironmentInitializerSpecsGitParam{
				RemoteUri: gitpod.F(spec.Git.RemoteURI.ValueString()),
			}
			if !spec.Git.CheckoutLocation.IsNull() && !spec.Git.CheckoutLocation.IsUnknown() {
				gitParam.CheckoutLocation = gitpod.F(spec.Git.CheckoutLocation.ValueString())
			}
			if !spec.Git.CloneTarget.IsNull() && !spec.Git.CloneTarget.IsUnknown() {
				gitParam.CloneTarget = gitpod.F(spec.Git.CloneTarget.ValueString())
			}
			if !spec.Git.TargetMode.IsNull() && !spec.Git.TargetMode.IsUnknown() {
				gitParam.TargetMode = gitpod.F(gitpod.EnvironmentInitializerSpecsGitTargetMode(spec.Git.TargetMode.ValueString()))
			}
			if !spec.Git.UpstreamRemoteURI.IsNull() && !spec.Git.UpstreamRemoteURI.IsUnknown() {
				gitParam.UpstreamRemoteUri = gitpod.F(spec.Git.UpstreamRemoteURI.ValueString())
			}

			specParam.Git = gitpod.F(gitParam)
		}

		specs = append(specs, specParam)
	}

	if diags.HasError() {
		return gitpod.EnvironmentInitializerParam{}, diags
	}

	return gitpod.EnvironmentInitializerParam{
		Specs: gitpod.F(specs),
	}, diags
}

func buildProjectPrebuildConfigurationParam(ctx context.Context, cfg *projectPrebuildConfigurationModel) (gitpod.ProjectPrebuildConfigurationParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	params := gitpod.ProjectPrebuildConfigurationParam{}

	if !cfg.Enabled.IsNull() && !cfg.Enabled.IsUnknown() {
		params.Enabled = gitpod.F(cfg.Enabled.ValueBool())
	}
	if !cfg.EnableJetbrainsWarmup.IsNull() && !cfg.EnableJetbrainsWarmup.IsUnknown() {
		params.EnableJetbrainsWarmup = gitpod.F(cfg.EnableJetbrainsWarmup.ValueBool())
	}
	if !cfg.EnvironmentClassIDs.IsNull() && !cfg.EnvironmentClassIDs.IsUnknown() {
		var values []string
		diags.Append(cfg.EnvironmentClassIDs.ElementsAs(ctx, &values, false)...)
		if diags.HasError() {
			return gitpod.ProjectPrebuildConfigurationParam{}, diags
		}
		params.EnvironmentClassIDs = gitpod.F(values)
	}
	if cfg.Executor != nil {
		params.Executor = gitpod.F(shared.SubjectParam{
			ID:        gitpod.F(cfg.Executor.ID.ValueString()),
			Principal: gitpod.F(shared.Principal(cfg.Executor.Principal.ValueString())),
		})
	}
	if !cfg.Timeout.IsNull() && !cfg.Timeout.IsUnknown() {
		params.Timeout = gitpod.F(cfg.Timeout.ValueString())
	}
	if cfg.Trigger != nil && cfg.Trigger.DailySchedule != nil {
		params.Trigger = gitpod.F(gitpod.ProjectPrebuildConfigurationTriggerParam{
			DailySchedule: gitpod.F(gitpod.ProjectPrebuildConfigurationTriggerDailyScheduleParam{
				HourUtc: gitpod.F(cfg.Trigger.DailySchedule.HourUTC.ValueInt64()),
			}),
		})
	}

	return params, diags
}

func buildRecommendedEditorsParam(ctx context.Context, editors map[string]projectRecommendedEditor) (gitpod.RecommendedEditorsParam, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string]gitpod.RecommendedEditorsEditorParam, len(editors))

	for alias, editor := range editors {
		var versions []string
		if !editor.Versions.IsNull() && !editor.Versions.IsUnknown() {
			diags.Append(editor.Versions.ElementsAs(ctx, &versions, false)...)
			if diags.HasError() {
				return gitpod.RecommendedEditorsParam{}, diags
			}
		}

		result[alias] = gitpod.RecommendedEditorsEditorParam{
			Versions: gitpod.F(versions),
		}
	}

	return gitpod.RecommendedEditorsParam{
		Editors: gitpod.F(result),
	}, diags
}

func mapProjectToModel(ctx context.Context, project gitpod.Project, prior projectModel) (projectModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := projectModel{
		ID:                   types.StringValue(project.ID),
		Name:                 mergeStringWithPrior(project.Metadata.Name, prior.Name),
		AutomationsFilePath:  mergeStringWithPrior(project.AutomationsFilePath, prior.AutomationsFilePath),
		DevcontainerFilePath: mergeStringWithPrior(project.DevcontainerFilePath, prior.DevcontainerFilePath),
		TechnicalDescription: mergeStringWithPrior(project.TechnicalDescription, prior.TechnicalDescription),
		DesiredPhase:         mergeStringWithPrior(string(project.DesiredPhase), prior.DesiredPhase),
		Initializer:          mapProjectInitializerToModel(project.Initializer, prior.Initializer),
	}

	prebuildPrior, prebuildPriorDiags := projectPrebuildConfigurationModelFromObject(ctx, prior.PrebuildConfiguration)
	diags.Append(prebuildPriorDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	prebuildConfig := mapProjectPrebuildConfigurationToModel(project.PrebuildConfiguration, prebuildPrior)
	prebuildValue, prebuildValueDiags := projectPrebuildConfigurationObjectValue(ctx, prebuildConfig)
	diags.Append(prebuildValueDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.PrebuildConfiguration = prebuildValue

	recommendedEditorsPrior, recommendedEditorsPriorDiags := projectRecommendedEditorsFromMap(ctx, prior.RecommendedEditors)
	diags.Append(recommendedEditorsPriorDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	recommendedEditors := mapRecommendedEditorsToModel(project.RecommendedEditors, recommendedEditorsPrior)
	recommendedEditorsValue, recommendedEditorsValueDiags := projectRecommendedEditorsMapValue(ctx, recommendedEditors)
	diags.Append(recommendedEditorsValueDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.RecommendedEditors = recommendedEditorsValue

	metadataValue, metadataDiags := projectMetadataObjectValue(ctx, &projectMetadataModel{
		Name:           stringValueOrNull(project.Metadata.Name),
		OrganizationID: stringValueOrNull(project.Metadata.OrganizationID),
		CreatedAt:      timeValueOrNull(project.Metadata.CreatedAt),
		UpdatedAt:      timeValueOrNull(project.Metadata.UpdatedAt),
		Creator:        mapSubjectToModel(project.Metadata.Creator, nil),
	})
	diags.Append(metadataDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.Metadata = metadataValue

	usedByValue, usedByDiags := projectUsedByObjectValue(ctx, &projectUsedByModel{
		TotalSubjects: types.Int64Value(project.UsedBy.TotalSubjects),
		Subjects:      mapSubjectsToModel(project.UsedBy.Subjects),
	})
	diags.Append(usedByDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.UsedBy = usedByValue

	return state, diags
}

func mapProjectInitializerToModel(initializer gitpod.EnvironmentInitializer, prior *projectInitializerModel) *projectInitializerModel {
	if len(initializer.Specs) == 0 {
		return prior
	}

	specs := make([]projectInitializerSpecModel, 0, len(initializer.Specs))
	for idx, spec := range initializer.Specs {
		var priorSpec *projectInitializerSpecModel
		if prior != nil && idx < len(prior.Specs) {
			priorSpec = &prior.Specs[idx]
		}

		modelSpec := projectInitializerSpecModel{}
		if hasInitializerContextURL(spec.ContextURL) || (priorSpec != nil && priorSpec.ContextURL != nil) {
			modelSpec.ContextURL = &projectInitializerContextURLModel{
				URL: mergeStringWithPrior(spec.ContextURL.URL, priorContextURLValue(priorSpec)),
			}
		}
		if hasInitializerGit(spec.Git) || (priorSpec != nil && priorSpec.Git != nil) {
			modelSpec.Git = &projectInitializerGitModel{
				CheckoutLocation:  mergeStringWithPrior(spec.Git.CheckoutLocation, priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.CheckoutLocation })),
				CloneTarget:       mergeStringWithPrior(spec.Git.CloneTarget, priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.CloneTarget })),
				RemoteURI:         mergeStringWithPrior(spec.Git.RemoteUri, priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.RemoteURI })),
				TargetMode:        mergeStringWithPrior(string(spec.Git.TargetMode), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.TargetMode })),
				UpstreamRemoteURI: mergeStringWithPrior(spec.Git.UpstreamRemoteUri, priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.UpstreamRemoteURI })),
			}
		}

		specs = append(specs, modelSpec)
	}

	return &projectInitializerModel{Specs: specs}
}

func mapProjectPrebuildConfigurationToModel(cfg gitpod.ProjectPrebuildConfiguration, prior *projectPrebuildConfigurationModel) *projectPrebuildConfigurationModel {
	if !hasProjectPrebuildConfiguration(cfg) && prior == nil {
		return nil
	}

	if !hasProjectPrebuildConfiguration(cfg) && prior != nil {
		return prior
	}

	model := &projectPrebuildConfigurationModel{
		Enabled:               types.BoolValue(cfg.Enabled),
		EnableJetbrainsWarmup: types.BoolValue(cfg.EnableJetbrainsWarmup),
		EnvironmentClassIDs:   stringListValue(cfg.EnvironmentClassIDs),
		Timeout:               stringValueOrNull(cfg.Timeout),
	}

	if cfg.Executor.ID != "" || cfg.Executor.Principal != "" {
		model.Executor = mapSubjectToModel(cfg.Executor, nil)
	} else if prior != nil {
		model.Executor = prior.Executor
	}

	if cfg.Trigger.DailySchedule.HourUtc != 0 {
		model.Trigger = &projectPrebuildTriggerModel{
			DailySchedule: &projectPrebuildDailyScheduleModel{
				HourUTC: types.Int64Value(cfg.Trigger.DailySchedule.HourUtc),
			},
		}
	} else if prior != nil {
		model.Trigger = prior.Trigger
	}

	if model.EnvironmentClassIDs.IsNull() && prior != nil {
		model.EnvironmentClassIDs = prior.EnvironmentClassIDs
	}

	return model
}

func mapRecommendedEditorsToModel(editors gitpod.RecommendedEditors, prior map[string]projectRecommendedEditor) map[string]projectRecommendedEditor {
	if editors.Editors == nil {
		return prior
	}

	result := make(map[string]projectRecommendedEditor, len(editors.Editors))
	for alias, editor := range editors.Editors {
		result[alias] = projectRecommendedEditor{
			Versions: stringListValue(editor.Versions),
		}
	}

	return result
}

func mapSubjectToModel(subject shared.Subject, prior *projectSubjectModel) *projectSubjectModel {
	if subject.ID == "" && subject.Principal == "" && prior != nil {
		return prior
	}

	return &projectSubjectModel{
		ID:        mergeStringWithPrior(subject.ID, priorSubjectValue(prior, func(s *projectSubjectModel) types.String { return s.ID })),
		Principal: mergeStringWithPrior(string(subject.Principal), priorSubjectValue(prior, func(s *projectSubjectModel) types.String { return s.Principal })),
	}
}

func mapSubjectsToModel(subjects []shared.Subject) []projectSubjectModel {
	result := make([]projectSubjectModel, 0, len(subjects))
	for _, subject := range subjects {
		result = append(result, projectSubjectModel{
			ID:        stringValueOrNull(subject.ID),
			Principal: stringValueOrNull(string(subject.Principal)),
		})
	}
	return result
}

func hasProjectPrebuildConfiguration(cfg gitpod.ProjectPrebuildConfiguration) bool {
	return cfg.Enabled ||
		cfg.EnableJetbrainsWarmup ||
		len(cfg.EnvironmentClassIDs) > 0 ||
		cfg.Executor.ID != "" ||
		cfg.Executor.Principal != "" ||
		cfg.Timeout != "" ||
		cfg.Trigger.DailySchedule.HourUtc != 0
}

func hasInitializerContextURL(contextURL gitpod.EnvironmentInitializerSpecsContextURL) bool {
	return contextURL.URL != ""
}

func hasInitializerGit(git gitpod.EnvironmentInitializerSpecsGit) bool {
	return git.CheckoutLocation != "" ||
		git.CloneTarget != "" ||
		git.RemoteUri != "" ||
		git.TargetMode != "" ||
		git.UpstreamRemoteUri != ""
}

func mergeStringWithPrior(current string, prior types.String) types.String {
	if current != "" {
		return types.StringValue(current)
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		return prior
	}
	return types.StringNull()
}

func stringListValue(values []string) types.List {
	elems := make([]types.String, 0, len(values))
	for _, value := range values {
		elems = append(elems, types.StringValue(value))
	}

	list, _ := types.ListValueFrom(context.Background(), types.StringType, elems)
	return list
}

func timeValueOrNull(value time.Time) types.String {
	if value.IsZero() {
		return types.StringNull()
	}
	return types.StringValue(value.Format(time.RFC3339Nano))
}

func projectPrebuildConfigurationModelFromObject(ctx context.Context, value types.Object) (*projectPrebuildConfigurationModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var model projectPrebuildConfigurationModel
	diags.Append(value.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}

	return &model, diags
}

func projectRecommendedEditorsFromMap(ctx context.Context, value types.Map) (map[string]projectRecommendedEditor, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return nil, diags
	}

	var editors map[string]projectRecommendedEditor
	diags.Append(value.ElementsAs(ctx, &editors, false)...)
	return editors, diags
}

func projectPrebuildConfigurationObjectValue(ctx context.Context, model *projectPrebuildConfigurationModel) (types.Object, diag.Diagnostics) {
	if model == nil {
		return types.ObjectNull(projectPrebuildConfigurationAttrTypes()), nil
	}
	return types.ObjectValueFrom(ctx, projectPrebuildConfigurationAttrTypes(), model)
}

func projectRecommendedEditorsMapValue(ctx context.Context, editors map[string]projectRecommendedEditor) (types.Map, diag.Diagnostics) {
	if editors == nil {
		return types.MapNull(projectRecommendedEditorObjectType()), nil
	}
	return types.MapValueFrom(ctx, projectRecommendedEditorObjectType(), editors)
}

func projectMetadataObjectValue(ctx context.Context, model *projectMetadataModel) (types.Object, diag.Diagnostics) {
	if model == nil {
		return types.ObjectNull(projectMetadataAttrTypes()), nil
	}
	return types.ObjectValueFrom(ctx, projectMetadataAttrTypes(), model)
}

func projectUsedByObjectValue(ctx context.Context, model *projectUsedByModel) (types.Object, diag.Diagnostics) {
	if model == nil {
		return types.ObjectNull(projectUsedByAttrTypes()), nil
	}
	return types.ObjectValueFrom(ctx, projectUsedByAttrTypes(), model)
}

func projectSubjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":        types.StringType,
		"principal": types.StringType,
	}
}

func projectSubjectObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: projectSubjectAttrTypes()}
}

func projectPrebuildDailyScheduleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"hour_utc": types.Int64Type,
	}
}

func projectPrebuildTriggerAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"daily_schedule": types.ObjectType{AttrTypes: projectPrebuildDailyScheduleAttrTypes()},
	}
}

func projectPrebuildConfigurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"enabled":                 types.BoolType,
		"enable_jetbrains_warmup": types.BoolType,
		"environment_class_ids":   types.ListType{ElemType: types.StringType},
		"executor":                projectSubjectObjectType(),
		"timeout":                 types.StringType,
		"trigger":                 types.ObjectType{AttrTypes: projectPrebuildTriggerAttrTypes()},
	}
}

func projectRecommendedEditorAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"versions": types.ListType{ElemType: types.StringType},
	}
}

func projectRecommendedEditorObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: projectRecommendedEditorAttrTypes()}
}

func projectMetadataAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":            types.StringType,
		"organization_id": types.StringType,
		"created_at":      types.StringType,
		"updated_at":      types.StringType,
		"creator":         projectSubjectObjectType(),
	}
}

func projectUsedByAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"total_subjects": types.Int64Type,
		"subjects":       types.ListType{ElemType: projectSubjectObjectType()},
	}
}

func priorContextURLValue(prior *projectInitializerSpecModel) types.String {
	if prior == nil || prior.ContextURL == nil {
		return types.StringNull()
	}
	return prior.ContextURL.URL
}

func priorGitValue(prior *projectInitializerSpecModel, fn func(*projectInitializerGitModel) types.String) types.String {
	if prior == nil || prior.Git == nil {
		return types.StringNull()
	}
	return fn(prior.Git)
}

func priorSubjectValue(prior *projectSubjectModel, fn func(*projectSubjectModel) types.String) types.String {
	if prior == nil {
		return types.StringNull()
	}
	return fn(prior)
}
