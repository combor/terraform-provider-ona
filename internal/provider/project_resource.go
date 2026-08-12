package provider

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResource struct {
	client *sdk.Client
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
				MarkdownDescription: "Human-readable project name. The API accepts 1 to 80 characters.",
				Validators:          []validator.String{stringvalidator.UTF8LengthBetween(1, 80)},
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
						MarkdownDescription: "Initializer specs. Each entry defines exactly one of `context_url` or `git`; use separate entries to combine them.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"context_url": schema.SingleNestedAttribute{
									Optional:            true,
									MarkdownDescription: "URL used to initialize the project context.",
									// Each spec entry carries exactly one initializer; use
									// separate entries to combine them.
									Validators: []validator.Object{
										objectvalidator.ExactlyOneOf(
											path.MatchRelative().AtParent().AtName("context_url"),
											path.MatchRelative().AtParent().AtName("git"),
										),
									},
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
											Computed:            true,
											MarkdownDescription: "Relative checkout path inside the environment.",
										},
										"clone_target": schema.StringAttribute{
											Optional:            true,
											Computed:            true,
											MarkdownDescription: "Clone target interpreted according to `target_mode`.",
										},
										"remote_uri": schema.StringAttribute{
											Required:            true,
											MarkdownDescription: "Git remote URI.",
										},
										"target_mode": schema.StringAttribute{
											Optional:            true,
											Computed:            true,
											MarkdownDescription: "Git clone target mode.",
											Validators:          enumValidators(v1.GitInitializer_CloneTargetMode_value),
										},
										"upstream_remote_uri": schema.StringAttribute{
											Optional:            true,
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
								Validators:          enumValidators(v1.Principal_value),
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
										Validators:          []validator.Int64{int64validator.Between(0, 23)},
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
	client, ok := clientFromProviderData(req.ProviderData, &resp.Diagnostics)
	if !ok {
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

	createResp, err := r.client.Services.Project.CreateProject(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project", err.Error())
		return
	}

	project := createResp.Msg.GetProject()

	recommendedEditorsModel, recommendedEditorsDiags := projectRecommendedEditorsFromMap(ctx, plan.RecommendedEditors)
	resp.Diagnostics.Append(recommendedEditorsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if recommendedEditorsModel != nil {
		createdStatePlan := plan
		createdStatePlan.RecommendedEditors = types.MapNull(projectRecommendedEditorObjectType())

		state, diags := mapProjectToModel(ctx, project, createdStatePlan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}

		updateParams := &v1.UpdateProjectRequest{
			ProjectId: project.GetId(),
		}
		recommendedEditors, updateDiags := buildRecommendedEditorsParam(ctx, recommendedEditorsModel)
		resp.Diagnostics.Append(updateDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateParams.RecommendedEditors = recommendedEditors

		updateResp, err := r.client.Services.Project.UpdateProject(ctx, connect.NewRequest(updateParams))
		if err != nil {
			resp.Diagnostics.AddWarning(
				"Project created without recommended editors",
				fmt.Sprintf(
					"Project %s was created successfully, but setting recommended_editors failed: %s. State was saved without recommended_editors so a future plan/apply can retry reconciliation.",
					project.GetId(),
					err.Error(),
				),
			)
			return
		}

		project = updateResp.Msg.GetProject()
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

	getResp, err := r.client.Services.Project.GetProject(ctx, connect.NewRequest(&v1.GetProjectRequest{
		ProjectId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read project", err.Error())
		return
	}

	newState, diags := mapProjectToModel(ctx, getResp.Msg.GetProject(), state)
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

	updateResp, err := r.client.Services.Project.UpdateProject(ctx, connect.NewRequest(params))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update project", err.Error())
		return
	}

	state, diags := mapProjectToModel(ctx, updateResp.Msg.GetProject(), plan)
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

	_, err := r.client.Services.Project.DeleteProject(ctx, connect.NewRequest(&v1.DeleteProjectRequest{
		ProjectId: state.ID.ValueString(),
	}))
	if err != nil {
		if isAPINotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Failed to delete project", err.Error())
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func buildProjectNewParams(ctx context.Context, plan projectModel) (*v1.CreateProjectRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	initializer, initDiags := buildEnvironmentInitializerParam(plan.Initializer)
	diags.Append(initDiags...)
	if diags.HasError() {
		return nil, diags
	}

	params := &v1.CreateProjectRequest{
		Name:        plan.Name.ValueString(),
		Initializer: initializer,
	}

	if !plan.AutomationsFilePath.IsNull() && !plan.AutomationsFilePath.IsUnknown() {
		params.AutomationsFilePath = plan.AutomationsFilePath.ValueString()
	}
	if !plan.DevcontainerFilePath.IsNull() && !plan.DevcontainerFilePath.IsUnknown() {
		params.DevcontainerFilePath = plan.DevcontainerFilePath.ValueString()
	}
	if !plan.TechnicalDescription.IsNull() && !plan.TechnicalDescription.IsUnknown() {
		params.TechnicalDescription = plan.TechnicalDescription.ValueString()
	}
	prebuildModel, prebuildModelDiags := projectPrebuildConfigurationModelFromObject(ctx, plan.PrebuildConfiguration)
	diags.Append(prebuildModelDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if prebuildModel != nil {
		prebuild, prebuildDiags := buildProjectPrebuildConfigurationParam(ctx, prebuildModel)
		diags.Append(prebuildDiags...)
		if diags.HasError() {
			return nil, diags
		}
		params.PrebuildConfiguration = prebuild
	}

	return params, diags
}

func buildProjectUpdateParams(ctx context.Context, plan projectModel) (*v1.UpdateProjectRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	params := &v1.UpdateProjectRequest{
		ProjectId: plan.ID.ValueString(),
		Name:      plan.Name.ValueStringPointer(),
	}

	initializer, initDiags := buildEnvironmentInitializerParam(plan.Initializer)
	diags.Append(initDiags...)
	if diags.HasError() {
		return nil, diags
	}
	params.Initializer = initializer

	if !plan.AutomationsFilePath.IsNull() && !plan.AutomationsFilePath.IsUnknown() {
		params.AutomationsFilePath = plan.AutomationsFilePath.ValueStringPointer()
	}
	if !plan.DevcontainerFilePath.IsNull() && !plan.DevcontainerFilePath.IsUnknown() {
		params.DevcontainerFilePath = plan.DevcontainerFilePath.ValueStringPointer()
	}
	if !plan.TechnicalDescription.IsNull() && !plan.TechnicalDescription.IsUnknown() {
		params.TechnicalDescription = plan.TechnicalDescription.ValueStringPointer()
	}
	prebuildModel, prebuildModelDiags := projectPrebuildConfigurationModelFromObject(ctx, plan.PrebuildConfiguration)
	diags.Append(prebuildModelDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if prebuildModel != nil {
		prebuild, prebuildDiags := buildProjectPrebuildConfigurationParam(ctx, prebuildModel)
		diags.Append(prebuildDiags...)
		if diags.HasError() {
			return nil, diags
		}
		params.PrebuildConfiguration = prebuild
	}
	recommendedEditorsModel, recommendedEditorsDiags := projectRecommendedEditorsFromMap(ctx, plan.RecommendedEditors)
	diags.Append(recommendedEditorsDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if recommendedEditorsModel != nil {
		recommendedEditors, recDiags := buildRecommendedEditorsParam(ctx, recommendedEditorsModel)
		diags.Append(recDiags...)
		if diags.HasError() {
			return nil, diags
		}
		params.RecommendedEditors = recommendedEditors
	}

	return params, diags
}

func buildEnvironmentInitializerParam(initializer *projectInitializerModel) (*v1.EnvironmentInitializer, diag.Diagnostics) {
	var diags diag.Diagnostics

	if initializer == nil || len(initializer.Specs) == 0 {
		diags.AddError("Missing initializer specs", "initializer.specs must contain at least one entry.")
		return nil, diags
	}

	specs := make([]*v1.EnvironmentInitializer_Spec, 0, len(initializer.Specs))
	for idx, spec := range initializer.Specs {
		if spec.ContextURL == nil && spec.Git == nil {
			diags.AddError("Invalid initializer spec",
				fmt.Sprintf("initializer.specs[%d] must set at least one of context_url or git.", idx))
			continue
		}
		if spec.ContextURL != nil && spec.Git != nil {
			diags.AddError("Invalid initializer spec",
				fmt.Sprintf("initializer.specs[%d] must set exactly one of context_url or git, not both.", idx))
			continue
		}

		specParam := &v1.EnvironmentInitializer_Spec{}
		if spec.ContextURL != nil {
			specParam.Spec = &v1.EnvironmentInitializer_Spec_ContextUrl{
				ContextUrl: &v1.ContextURLInitializer{
					Url: spec.ContextURL.URL.ValueString(),
				},
			}
		}
		if spec.Git != nil {
			gitParam := &v1.GitInitializer{
				RemoteUri: spec.Git.RemoteURI.ValueString(),
			}
			if !spec.Git.CheckoutLocation.IsNull() && !spec.Git.CheckoutLocation.IsUnknown() {
				gitParam.CheckoutLocation = spec.Git.CheckoutLocation.ValueString()
			}
			if !spec.Git.CloneTarget.IsNull() && !spec.Git.CloneTarget.IsUnknown() {
				gitParam.CloneTarget = spec.Git.CloneTarget.ValueString()
			}
			if !spec.Git.TargetMode.IsNull() && !spec.Git.TargetMode.IsUnknown() {
				gitParam.TargetMode = enumValue[v1.GitInitializer_CloneTargetMode](
					fmt.Sprintf("initializer.specs[%d].git.target_mode", idx),
					spec.Git.TargetMode.ValueString(), v1.GitInitializer_CloneTargetMode_value, &diags)
			}
			if !spec.Git.UpstreamRemoteURI.IsNull() && !spec.Git.UpstreamRemoteURI.IsUnknown() {
				gitParam.UpstreamRemoteUri = spec.Git.UpstreamRemoteURI.ValueString()
			}

			specParam.Spec = &v1.EnvironmentInitializer_Spec_Git{Git: gitParam}
		}

		specs = append(specs, specParam)
	}

	if diags.HasError() {
		return nil, diags
	}

	return &v1.EnvironmentInitializer{
		Specs: specs,
	}, diags
}

func buildProjectPrebuildConfigurationParam(ctx context.Context, cfg *projectPrebuildConfigurationModel) (*v1.ProjectPrebuildConfiguration, diag.Diagnostics) {
	var diags diag.Diagnostics
	params := &v1.ProjectPrebuildConfiguration{}

	if !cfg.Enabled.IsNull() && !cfg.Enabled.IsUnknown() {
		params.Enabled = cfg.Enabled.ValueBool()
	}
	if !cfg.EnableJetbrainsWarmup.IsNull() && !cfg.EnableJetbrainsWarmup.IsUnknown() {
		params.EnableJetbrainsWarmup = cfg.EnableJetbrainsWarmup.ValueBool()
	}
	if !cfg.EnvironmentClassIDs.IsNull() && !cfg.EnvironmentClassIDs.IsUnknown() {
		var values []string
		diags.Append(cfg.EnvironmentClassIDs.ElementsAs(ctx, &values, false)...)
		if diags.HasError() {
			return nil, diags
		}
		params.EnvironmentClassIds = values
	}
	if cfg.Executor != nil {
		params.Executor = &v1.Subject{
			Id: cfg.Executor.ID.ValueString(),
			Principal: enumValue[v1.Principal]("prebuild_configuration.executor.principal",
				cfg.Executor.Principal.ValueString(), v1.Principal_value, &diags),
		}
	}
	if !cfg.Timeout.IsNull() && !cfg.Timeout.IsUnknown() {
		timeout, err := time.ParseDuration(cfg.Timeout.ValueString())
		if err != nil {
			diags.AddError("Invalid prebuild timeout",
				fmt.Sprintf("prebuild_configuration.timeout must be a valid duration such as `3600s`: %s", err.Error()))
			return nil, diags
		}
		params.Timeout = durationpb.New(timeout)
	}
	if cfg.Trigger != nil && cfg.Trigger.DailySchedule != nil {
		hourUTC, ok := validatedHour("prebuild_configuration.trigger.daily_schedule.hour_utc",
			cfg.Trigger.DailySchedule.HourUTC, &diags)
		if !ok {
			return nil, diags
		}

		params.Trigger = &v1.PrebuildTrigger{
			Trigger: &v1.PrebuildTrigger_DailySchedule_{
				DailySchedule: &v1.PrebuildTrigger_DailySchedule{
					HourUtc: int32(hourUTC),
				},
			},
		}
	}

	return params, diags
}

func buildRecommendedEditorsParam(ctx context.Context, editors map[string]projectRecommendedEditor) (*v1.RecommendedEditors, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := make(map[string]*v1.EditorVersions, len(editors))

	for alias, editor := range editors {
		var versions []string
		if !editor.Versions.IsNull() && !editor.Versions.IsUnknown() {
			diags.Append(editor.Versions.ElementsAs(ctx, &versions, false)...)
			if diags.HasError() {
				return nil, diags
			}
		}

		result[alias] = &v1.EditorVersions{
			Versions: versions,
		}
	}

	return &v1.RecommendedEditors{
		Editors: result,
	}, diags
}

func mapProjectToModel(ctx context.Context, project *v1.Project, prior projectModel) (projectModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := projectModel{
		ID:                   types.StringValue(project.GetId()),
		Name:                 mergeStringWithPrior(project.GetMetadata().GetName(), prior.Name),
		AutomationsFilePath:  mergeStringWithPrior(project.GetAutomationsFilePath(), prior.AutomationsFilePath),
		DevcontainerFilePath: mergeStringWithPrior(project.GetDevcontainerFilePath(), prior.DevcontainerFilePath),
		TechnicalDescription: mergeStringWithPrior(project.GetTechnicalDescription(), prior.TechnicalDescription),
		DesiredPhase:         mergeStringWithPrior(enumString(project.GetDesiredPhase()), prior.DesiredPhase),
		Initializer:          mapProjectInitializerToModel(project.GetInitializer(), prior.Initializer),
	}

	prebuildPrior, prebuildPriorDiags := projectPrebuildConfigurationModelFromObject(ctx, prior.PrebuildConfiguration)
	diags.Append(prebuildPriorDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	prebuildConfig := mapProjectPrebuildConfigurationToModel(
		project.GetPrebuildConfiguration(),
		prebuildPrior,
	)
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
	recommendedEditors := mapRecommendedEditorsToModel(project.GetRecommendedEditors(), recommendedEditorsPrior)
	recommendedEditorsValue, recommendedEditorsValueDiags := projectRecommendedEditorsMapValue(ctx, recommendedEditors)
	diags.Append(recommendedEditorsValueDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.RecommendedEditors = recommendedEditorsValue

	metadataValue, metadataDiags := projectMetadataObjectValue(ctx, &projectMetadataModel{
		Name:           stringValueOrNull(project.GetMetadata().GetName()),
		OrganizationID: stringValueOrNull(project.GetMetadata().GetOrganizationId()),
		CreatedAt:      timeValueOrNull(project.GetMetadata().GetCreatedAt()),
		UpdatedAt:      timeValueOrNull(project.GetMetadata().GetUpdatedAt()),
		Creator:        mapSubjectToModel(project.GetMetadata().GetCreator(), nil),
	})
	diags.Append(metadataDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.Metadata = metadataValue

	usedByValue, usedByDiags := projectUsedByObjectValue(ctx, &projectUsedByModel{
		TotalSubjects: types.Int64Value(int64(project.GetUsedBy().GetTotalSubjects())),
		Subjects:      mapSubjectsToModel(project.GetUsedBy().GetSubjects()),
	})
	diags.Append(usedByDiags...)
	if diags.HasError() {
		return projectModel{}, diags
	}
	state.UsedBy = usedByValue

	return state, diags
}

func mapProjectInitializerToModel(initializer *v1.EnvironmentInitializer, prior *projectInitializerModel) *projectInitializerModel {
	if len(initializer.GetSpecs()) == 0 {
		return prior
	}

	specs := make([]projectInitializerSpecModel, 0, len(initializer.GetSpecs()))
	for idx, spec := range initializer.GetSpecs() {
		var priorSpec *projectInitializerSpecModel
		if prior != nil && idx < len(prior.Specs) {
			priorSpec = &prior.Specs[idx]
		}

		// spec is a oneof, so prior values may only fill in gaps within the
		// variant the API returned. Carrying the other variant over would write
		// a both-set state that buildEnvironmentInitializerParam rejects.
		modelSpec := projectInitializerSpecModel{}
		switch {
		case spec.GetContextUrl() != nil:
			modelSpec.ContextURL = &projectInitializerContextURLModel{
				URL: mergeStringWithPrior(spec.GetContextUrl().GetUrl(), priorContextURLValue(priorSpec)),
			}
		case spec.GetGit() != nil:
			git := spec.GetGit()
			modelSpec.Git = &projectInitializerGitModel{
				CheckoutLocation:  mergeStringWithPrior(git.GetCheckoutLocation(), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.CheckoutLocation })),
				CloneTarget:       mergeStringWithPrior(git.GetCloneTarget(), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.CloneTarget })),
				RemoteURI:         mergeStringWithPrior(git.GetRemoteUri(), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.RemoteURI })),
				TargetMode:        mergeStringWithPrior(enumString(git.GetTargetMode()), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.TargetMode })),
				UpstreamRemoteURI: mergeStringWithPrior(git.GetUpstreamRemoteUri(), priorGitValue(priorSpec, func(g *projectInitializerGitModel) types.String { return g.UpstreamRemoteURI })),
			}
		case priorSpec != nil:
			// The API returned neither variant; keep what state already had.
			modelSpec = *priorSpec
		}

		specs = append(specs, modelSpec)
	}

	return &projectInitializerModel{Specs: specs}
}

func mapProjectPrebuildConfigurationToModel(cfg *v1.ProjectPrebuildConfiguration, prior *projectPrebuildConfigurationModel) *projectPrebuildConfigurationModel {
	if cfg == nil {
		return knownProjectPrebuildConfiguration(prior)
	}

	prior = knownProjectPrebuildConfiguration(prior)

	model := &projectPrebuildConfigurationModel{
		// enabled and enable_jetbrains_warmup are plain proto3 bools, which carry
		// no presence: an omitted field is indistinguishable from false, so the
		// wire value is authoritative and there is nothing to fall back to.
		Enabled:               types.BoolValue(cfg.GetEnabled()),
		EnableJetbrainsWarmup: types.BoolValue(cfg.GetEnableJetbrainsWarmup()),
		EnvironmentClassIDs:   types.ListNull(types.StringType),
		Timeout:               types.StringNull(),
	}

	if cfg.GetEnvironmentClassIds() != nil {
		model.EnvironmentClassIDs = stringListValue(cfg.GetEnvironmentClassIds())
	} else if prior != nil {
		model.EnvironmentClassIDs = prior.EnvironmentClassIDs
	}

	if cfg.GetExecutor() != nil {
		model.Executor = mapSubjectToModel(cfg.GetExecutor(), nil)
	} else if prior != nil {
		model.Executor = prior.Executor
	}

	if cfg.GetTimeout() != nil {
		model.Timeout = timeoutValueWithPrior(cfg.GetTimeout(), priorTimeout(prior))
	} else if prior != nil {
		model.Timeout = prior.Timeout
	}

	model.Trigger = mapProjectPrebuildTriggerToModel(cfg.GetTrigger(), priorProjectPrebuildTrigger(prior))

	return model
}

func mapProjectPrebuildTriggerToModel(trigger *v1.PrebuildTrigger, prior *projectPrebuildTriggerModel) *projectPrebuildTriggerModel {
	if trigger == nil {
		return prior
	}

	return &projectPrebuildTriggerModel{
		DailySchedule: mapProjectPrebuildDailyScheduleToModel(trigger.GetDailySchedule(), priorProjectPrebuildDailySchedule(prior)),
	}
}

func mapProjectPrebuildDailyScheduleToModel(schedule *v1.PrebuildTrigger_DailySchedule, prior *projectPrebuildDailyScheduleModel) *projectPrebuildDailyScheduleModel {
	if schedule == nil {
		return prior
	}

	return &projectPrebuildDailyScheduleModel{
		HourUTC: types.Int64Value(int64(schedule.GetHourUtc())),
	}
}

// timeoutValueWithPrior keeps the configured spelling of a prebuild timeout
// when it denotes the same duration the API returned. timeout is
// optional-and-computed, so echoing a re-spelled but equivalent value (for
// example "3600s" for a configured "1h") would fail Terraform's
// consistent-result check.
func timeoutValueWithPrior(timeout *durationpb.Duration, prior types.String) types.String {
	if !prior.IsNull() && !prior.IsUnknown() {
		if configured, err := time.ParseDuration(prior.ValueString()); err == nil && configured == timeout.AsDuration() {
			return prior
		}
	}

	return stringValueOrNull(durationString(timeout))
}

func priorTimeout(prior *projectPrebuildConfigurationModel) types.String {
	if prior == nil {
		return types.StringNull()
	}

	return prior.Timeout
}

func priorProjectPrebuildTrigger(prior *projectPrebuildConfigurationModel) *projectPrebuildTriggerModel {
	if prior == nil {
		return nil
	}

	return prior.Trigger
}

func priorProjectPrebuildDailySchedule(prior *projectPrebuildTriggerModel) *projectPrebuildDailyScheduleModel {
	if prior == nil {
		return nil
	}

	return prior.DailySchedule
}

func knownProjectPrebuildConfiguration(prior *projectPrebuildConfigurationModel) *projectPrebuildConfigurationModel {
	if prior == nil {
		return nil
	}

	return &projectPrebuildConfigurationModel{
		Enabled:               knownBoolOrNull(prior.Enabled),
		EnableJetbrainsWarmup: knownBoolOrNull(prior.EnableJetbrainsWarmup),
		EnvironmentClassIDs:   knownStringListOrNull(prior.EnvironmentClassIDs),
		Executor:              knownProjectSubject(prior.Executor),
		Timeout:               knownStringOrNull(prior.Timeout),
		Trigger:               knownProjectPrebuildTrigger(prior.Trigger),
	}
}

func knownProjectPrebuildTrigger(prior *projectPrebuildTriggerModel) *projectPrebuildTriggerModel {
	if prior == nil {
		return nil
	}

	dailySchedule := knownProjectPrebuildDailySchedule(prior.DailySchedule)
	if dailySchedule == nil {
		return nil
	}

	return &projectPrebuildTriggerModel{
		DailySchedule: dailySchedule,
	}
}

func knownProjectPrebuildDailySchedule(prior *projectPrebuildDailyScheduleModel) *projectPrebuildDailyScheduleModel {
	if prior == nil || prior.HourUTC.IsUnknown() {
		return nil
	}

	return &projectPrebuildDailyScheduleModel{
		HourUTC: prior.HourUTC,
	}
}

func knownProjectSubject(prior *projectSubjectModel) *projectSubjectModel {
	if prior == nil || prior.ID.IsUnknown() || prior.Principal.IsUnknown() {
		return nil
	}

	return &projectSubjectModel{
		ID:        prior.ID,
		Principal: prior.Principal,
	}
}

func knownBoolOrNull(prior types.Bool) types.Bool {
	if prior.IsUnknown() {
		return types.BoolNull()
	}

	return prior
}

func knownStringOrNull(prior types.String) types.String {
	if prior.IsUnknown() {
		return types.StringNull()
	}

	return prior
}

func knownStringListOrNull(prior types.List) types.List {
	if prior.IsUnknown() {
		return types.ListNull(types.StringType)
	}

	return prior
}

func mapRecommendedEditorsToModel(editors *v1.RecommendedEditors, prior map[string]projectRecommendedEditor) map[string]projectRecommendedEditor {
	if editors.GetEditors() == nil {
		return prior
	}

	result := make(map[string]projectRecommendedEditor, len(editors.GetEditors()))
	for alias, editor := range editors.GetEditors() {
		result[alias] = projectRecommendedEditor{
			Versions: stringListValue(editor.GetVersions()),
		}
	}

	return result
}

func mapSubjectToModel(subject *v1.Subject, prior *projectSubjectModel) *projectSubjectModel {
	principal := enumString(subject.GetPrincipal())
	if subject.GetId() == "" && principal == "" && prior != nil {
		return prior
	}

	return &projectSubjectModel{
		ID:        mergeStringWithPrior(subject.GetId(), priorSubjectValue(prior, func(s *projectSubjectModel) types.String { return s.ID })),
		Principal: mergeStringWithPrior(principal, priorSubjectValue(prior, func(s *projectSubjectModel) types.String { return s.Principal })),
	}
}

func mapSubjectsToModel(subjects []*v1.Subject) []projectSubjectModel {
	result := make([]projectSubjectModel, 0, len(subjects))
	for _, subject := range subjects {
		result = append(result, projectSubjectModel{
			ID:        stringValueOrNull(subject.GetId()),
			Principal: stringValueOrNull(enumString(subject.GetPrincipal())),
		})
	}
	return result
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
