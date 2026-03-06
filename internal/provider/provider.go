package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/gitpod-io/gitpod-sdk-go/option"
)

var _ provider.Provider = &onaProvider{}

type onaProvider struct {
	version string
}

type onaProviderModel struct {
	APIKey         types.String `tfsdk:"api_key"`
	BaseURL        types.String `tfsdk:"base_url"`
	MaxRetries     types.Int64  `tfsdk:"max_retries"`
	RequestTimeout types.String `tfsdk:"request_timeout"`
}

func (p *onaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ona"
	resp.Version = p.version
}

func (p *onaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "API key. Falls back to `GITPOD_API_KEY` env var.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "API base URL. Falls back to `GITPOD_BASE_URL` env var. Defaults to `https://app.gitpod.io/api`.",
			},
			"max_retries": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of retries per request. Defaults to the SDK default (2). Set to `0` to disable retries.",
			},
			"request_timeout": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Per-attempt request timeout as a Go duration (for example `20s` or `2m`). If unset, requests have no SDK-level timeout.",
			},
		},
	}
}

func (p *onaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config onaProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := config.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("GITPOD_API_KEY")
	}
	if apiKey == "" {
		resp.Diagnostics.AddError("Missing API Key", "Set api_key in provider config or GITPOD_API_KEY env var.")
		return
	}

	baseURL := config.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("GITPOD_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://app.gitpod.io/api"
	}

	clientOptions := []option.RequestOption{
		option.WithBearerToken(apiKey),
		option.WithBaseURL(baseURL),
	}

	if !config.MaxRetries.IsNull() {
		if config.MaxRetries.IsUnknown() {
			resp.Diagnostics.AddError(
				"Invalid max_retries",
				"Provider attribute max_retries must be known during provider configuration.",
			)
			return
		}

		maxRetries := config.MaxRetries.ValueInt64()
		if maxRetries < 0 {
			resp.Diagnostics.AddError(
				"Invalid max_retries",
				"Provider attribute max_retries must be greater than or equal to 0.",
			)
			return
		}

		clientOptions = append(clientOptions, option.WithMaxRetries(int(maxRetries)))
	}

	if !config.RequestTimeout.IsNull() {
		if config.RequestTimeout.IsUnknown() {
			resp.Diagnostics.AddError(
				"Invalid request_timeout",
				"Provider attribute request_timeout must be known during provider configuration.",
			)
			return
		}

		requestTimeout, err := time.ParseDuration(config.RequestTimeout.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid request_timeout",
				fmt.Sprintf("Provider attribute request_timeout must be a valid Go duration string: %s", err.Error()),
			)
			return
		}
		if requestTimeout <= 0 {
			resp.Diagnostics.AddError(
				"Invalid request_timeout",
				"Provider attribute request_timeout must be greater than 0.",
			)
			return
		}

		clientOptions = append(clientOptions, option.WithRequestTimeout(requestTimeout))
	}

	client := gitpod.NewClient(clientOptions...)

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *onaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewRunnerResource,
	}
}

func (p *onaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewAuthenticatedIdentityDataSource,
		NewGroupDataSource,
		NewGroupsDataSource,
		NewProjectDataSource,
		NewRunnerEnvironmentClassesDataSource,
		NewRunnerDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &onaProvider{version: version}
	}
}
