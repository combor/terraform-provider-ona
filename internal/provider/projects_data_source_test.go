package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectsDataSourceRead_SendsFiltersAndDrainsPagesInAPIOrder(t *testing.T) {
	client := &projectsListClient{pages: [][]*v1.Project{
		{testProject("project-b", "Beta"), testProject("project-a", "Alpha")},
		{testProject("project-c", "Gamma")},
	}}

	state, diags := readProjectsDataSource(t, client, projectsDataSourceModel{
		Search:      types.StringValue("backend"),
		ProjectIDs:  []types.String{types.StringValue("project-a"), types.StringValue("project-b")},
		RunnerIDs:   []types.String{types.StringValue("runner-1")},
		RunnerKinds: []types.String{types.StringValue("RUNNER_KIND_REMOTE")},
		RemoteURIs:  []types.String{types.StringValue("https://github.com/combor/terraform-provider-ona")},
		CreatorIDs:  []types.String{types.StringValue("user-1")},
	})
	require.False(t, diags.HasError(), "%v", diags)

	require.Len(t, client.requests, 2)
	for i, req := range client.requests {
		assert.Equal(t, int32(100), req.GetPagination().GetPageSize())
		assert.Equal(t, "backend", req.GetFilter().GetSearch())
		assert.Equal(t, []string{"project-a", "project-b"}, req.GetFilter().GetProjectIds())
		assert.Equal(t, []string{"runner-1"}, req.GetFilter().GetRunnerIds())
		assert.Equal(t, []v1.RunnerKind{v1.RunnerKind_RUNNER_KIND_REMOTE}, req.GetFilter().GetRunnerKinds())
		assert.Equal(t, []string{"https://github.com/combor/terraform-provider-ona"}, req.GetFilter().GetSpecRemoteUris())
		assert.Equal(t, []string{"user-1"}, req.GetFilter().GetCreatorIds())
		assert.Equal(t, "id", req.GetSort().GetField(), "request %d", i)
		assert.Equal(t, v1.SortOrder_SORT_ORDER_ASC, req.GetSort().GetOrder(), "request %d", i)
		assert.Nil(t, req.GetCount())
	}
	assert.Empty(t, client.requests[0].GetPagination().GetToken())
	assert.Equal(t, "page-1", client.requests[1].GetPagination().GetToken())

	require.Len(t, state.Projects, 3)
	assert.Equal(t, "project-b", state.Projects[0].ID.ValueString())
	assert.Equal(t, "Beta", state.Projects[0].Name.ValueString())
	assert.Equal(t, "project-a", state.Projects[1].ID.ValueString())
	assert.Equal(t, "project-c", state.Projects[2].ID.ValueString())
	assert.Equal(t, "backend", state.Search.ValueString())
	assert.True(t, state.TotalCount.IsNull())
	assert.True(t, state.TotalCountRelation.IsNull())
}

func TestProjectsDataSourceRead_LimitStopsPaging(t *testing.T) {
	client := &projectsListClient{pages: [][]*v1.Project{
		{testProject("project-1", "One"), testProject("project-2", "Two")},
		{testProject("project-3", "Three"), testProject("project-4", "Four")},
		{testProject("project-5", "Five")},
	}}

	state, diags := readProjectsDataSource(t, client, projectsDataSourceModel{
		Limit: types.Int64Value(3),
	})
	require.False(t, diags.HasError(), "%v", diags)

	require.Len(t, client.requests, 2)
	assert.Equal(t, int32(3), client.requests[0].GetPagination().GetPageSize())
	assert.Equal(t, int32(1), client.requests[1].GetPagination().GetPageSize())

	require.Len(t, state.Projects, 3)
	assert.Equal(t, "project-3", state.Projects[2].ID.ValueString())
	assert.Equal(t, int64(3), state.Limit.ValueInt64())
}

func TestProjectsDataSourceRead_IncludeCountAndSort(t *testing.T) {
	client := &projectsListClient{
		pages: [][]*v1.Project{
			{testProject("project-1", "One")},
			{testProject("project-2", "Two")},
		},
		count: &v1.CountResponse{Value: 1000, Relation: v1.CountResponseRelation_COUNT_RESPONSE_RELATION_GTE},
	}

	state, diags := readProjectsDataSource(t, client, projectsDataSourceModel{
		Sort: &projectsSortModel{
			Field: types.StringValue("popularity"),
			Order: types.StringValue("SORT_ORDER_DESC"),
		},
		IncludeCount: types.BoolValue(true),
	})
	require.False(t, diags.HasError(), "%v", diags)

	require.Len(t, client.requests, 2)
	assert.True(t, client.requests[0].GetCount().GetInclude())
	assert.Nil(t, client.requests[1].GetCount())
	for _, req := range client.requests {
		assert.Equal(t, "popularity", req.GetSort().GetField())
		assert.Equal(t, v1.SortOrder_SORT_ORDER_DESC, req.GetSort().GetOrder())
	}

	require.Len(t, state.Projects, 2)
	assert.Equal(t, int64(1000), state.TotalCount.ValueInt64())
	assert.Equal(t, "COUNT_RESPONSE_RELATION_GTE", state.TotalCountRelation.ValueString())
}

func TestProjectsDataSourceRead_NoMatchesSetsEmptyList(t *testing.T) {
	client := &projectsListClient{pages: [][]*v1.Project{{}}}

	state, diags := readProjectsDataSource(t, client, projectsDataSourceModel{})
	require.False(t, diags.HasError(), "%v", diags)

	require.NotNil(t, state.Projects)
	assert.Empty(t, state.Projects)
	assert.Nil(t, state.ProjectIDs)
	assert.True(t, state.Search.IsNull())
}

func TestProjectsDataSourceRead_ListErrorIsReported(t *testing.T) {
	client := &projectsListClient{err: connect.NewError(connect.CodePermissionDenied, fmt.Errorf("denied"))}

	_, diags := readProjectsDataSource(t, client, projectsDataSourceModel{})

	require.True(t, diags.HasError())
	assert.Equal(t, "Failed to list projects", diags[0].Summary())
}

func TestProjectsDataSourceSchema_SearchLengthCountsCharacters(t *testing.T) {
	ctx := t.Context()
	var schemaResp datasource.SchemaResponse
	NewProjectsDataSource().Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), "%v", schemaResp.Diagnostics)

	search, ok := schemaResp.Schema.Attributes["search"].(schema.StringAttribute)
	require.True(t, ok)

	for value, wantError := range map[string]bool{
		strings.Repeat("é", 256): false,
		strings.Repeat("é", 257): true,
	} {
		resp := &validator.StringResponse{}
		for _, v := range search.Validators {
			v.ValidateString(ctx, validator.StringRequest{Path: path.Root("search"), ConfigValue: types.StringValue(value)}, resp)
		}
		assert.Equal(t, wantError, resp.Diagnostics.HasError(), "%d characters: %v", len([]rune(value)), resp.Diagnostics)
	}
}

// readProjectsDataSource runs Read with config and returns the resulting state.
func readProjectsDataSource(t *testing.T, client *projectsListClient, config projectsDataSourceModel) (projectsDataSourceModel, diag.Diagnostics) {
	t.Helper()
	ctx := t.Context()

	d := &projectsDataSource{client: &sdk.Client{Services: sdk.ServiceClients{Project: client}}}
	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), "%v", schemaResp.Diagnostics)

	// Encode the config through State since Config has no Set method.
	configValue := tfsdk.State{Schema: schemaResp.Schema}
	diags := configValue.Set(ctx, &config)
	require.False(t, diags.HasError(), "%v", diags)

	resp := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: configValue.Raw}}, &resp)
	if resp.Diagnostics.HasError() {
		return projectsDataSourceModel{}, resp.Diagnostics
	}

	var state projectsDataSourceModel
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	return state, resp.Diagnostics
}

func testProject(id, name string) *v1.Project {
	return &v1.Project{Id: id, Metadata: &v1.ProjectMetadata{Name: name}}
}

// projectsListClient serves pages in order, linking them with "page-N" tokens.
type projectsListClient struct {
	v1connect.ProjectServiceClient
	pages    [][]*v1.Project
	count    *v1.CountResponse
	err      error
	requests []*v1.ListProjectsRequest
}

func (c *projectsListClient) ListProjects(_ context.Context, req *connect.Request[v1.ListProjectsRequest]) (*connect.Response[v1.ListProjectsResponse], error) {
	c.requests = append(c.requests, req.Msg)
	if c.err != nil {
		return nil, c.err
	}

	page := len(c.requests) - 1
	if page >= len(c.pages) {
		return nil, fmt.Errorf("unexpected request for page %d", page)
	}

	resp := &v1.ListProjectsResponse{Projects: c.pages[page], Pagination: &v1.PaginationResponse{}}
	if page+1 < len(c.pages) {
		resp.Pagination.NextToken = fmt.Sprintf("page-%d", page+1)
	}
	if req.Msg.GetCount().GetInclude() {
		resp.Count = c.count
	}
	return connect.NewResponse(resp), nil
}
