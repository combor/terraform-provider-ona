package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gitpod-io/gitpod-sdk-go/sdk"
)

const defaultBaseURL = "https://app.ona.com/api"

// apiKeyEnvMu serialises the os.Setenv/sdk.NewFromEnv/restore sequence in
// Configure. sdk.NewFromEnv is the only usable constructor — sdk.New takes an
// internal type — and it reads the API key from the process environment, which
// aliased provider instances could otherwise race on.
var apiKeyEnvMu sync.Mutex

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

func maxRuntimeInt64() int64 {
	return int64(^uint(0) >> 1)
}

func int64ToIntChecked(value int64, maxValue int64) (int, error) {
	if value > maxValue {
		return 0, fmt.Errorf("too large for this runtime (max %d)", maxValue)
	}

	return int(value), nil
}

func (p *onaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "ona"
	resp.Version = p.version
}

func (p *onaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for managing [Gitpod](https://gitpod.io) resources on [ona.com](https://ona.com).",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "API key. Falls back to `GITPOD_API_KEY` env var.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "API base URL. Falls back to `GITPOD_BASE_URL` env var. Defaults to `https://app.ona.com/api`.",
			},
			"max_retries": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of retries per request. Defaults to `2`. Set to `0` to disable retries.",
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
		baseURL = defaultBaseURL
	}

	maxRetries := defaultMaxRetries

	if !config.MaxRetries.IsNull() {
		if config.MaxRetries.IsUnknown() {
			resp.Diagnostics.AddError(
				"Invalid max_retries",
				"Provider attribute max_retries must be known during provider configuration.",
			)
			return
		}

		configured := config.MaxRetries.ValueInt64()
		if configured < 0 {
			resp.Diagnostics.AddError(
				"Invalid max_retries",
				"Provider attribute max_retries must be greater than or equal to 0.",
			)
			return
		}

		checked, err := int64ToIntChecked(configured, maxRuntimeInt64())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid max_retries",
				fmt.Sprintf("Provider attribute max_retries is %s.", err.Error()),
			)
			return
		}

		maxRetries = checked
	}

	transport := &retryTransport{base: http.DefaultTransport, maxRetries: maxRetries}

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

		// Applied per attempt inside the transport rather than as
		// http.Client.Timeout, which would span every retry and backoff sleep.
		transport.perAttemptTimeout = requestTimeout
	}

	client, err := newSDKClient(apiKey, baseURL, &http.Client{Transport: transport})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create API client", err.Error())
		return
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

// newSDKClient builds the SDK client from an explicit API key. sdk.NewFromEnv
// reads the key from the environment and captures it eagerly in a static token
// source, so the previous value can be restored as soon as it returns.
func newSDKClient(apiKey, baseURL string, httpClient *http.Client) (*sdk.Client, error) {
	apiKeyEnvMu.Lock()
	defer apiKeyEnvMu.Unlock()

	previous, existed := os.LookupEnv(sdk.APIKeyEnvVar)
	if err := os.Setenv(sdk.APIKeyEnvVar, apiKey); err != nil {
		return nil, err
	}
	defer func() {
		if existed {
			_ = os.Setenv(sdk.APIKeyEnvVar, previous)
			return
		}
		_ = os.Unsetenv(sdk.APIKeyEnvVar)
	}()

	return sdk.NewFromEnv(sdk.WithBaseURL(baseURL), sdk.WithHTTPClient(httpClient))
}

func (p *onaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGroupResource,
		NewOrganizationPoliciesResource,
		NewProjectPolicyResource,
		NewProjectResource,
		NewRunnerResource,
		NewRunnerEnvironmentClassResource,
		NewRunnerPolicyResource,
		NewRunnerScmIntegrationResource,
		NewSecretResource,
		NewSecurityPolicyResource,
		NewServiceAccountResource,
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
		NewRunnersDataSource,
		NewRunnerTokenDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &onaProvider{version: version}
	}
}
