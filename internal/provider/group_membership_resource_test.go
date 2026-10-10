package provider

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/gitpod-io/gitpod-sdk-go/sdk"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	"github.com/gitpod-io/gitpod-sdk-go/v1/v1connect"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupMembershipResourceLifecycle(t *testing.T) {
	for _, principal := range []v1.Principal{v1.Principal_PRINCIPAL_USER, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT} {
		t.Run(principal.String(), func(t *testing.T) {
			ctx := t.Context()
			client := &groupMembershipClient{created: &v1.GroupMembership{Id: "membership-1"}}
			r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
			model := groupMembershipModel{ID: types.StringUnknown(), GroupID: types.StringValue("group-1")}
			if principal == v1.Principal_PRINCIPAL_USER {
				model.UserID = types.StringValue("member-1")
			} else {
				model.ServiceAccountID = types.StringValue("member-1")
			}
			state := groupMembershipTestState(t, model)
			createResp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
			require.False(t, createResp.Diagnostics.HasError(), "%v", createResp.Diagnostics)
			require.Len(t, client.creates, 1)
			assert.Equal(t, "group-1", client.creates[0].GetGroupId())
			assert.Equal(t, "member-1", client.creates[0].GetSubject().GetId())
			assert.Equal(t, principal, client.creates[0].GetSubject().GetPrincipal())

			var created groupMembershipModel
			require.False(t, createResp.State.Get(ctx, &created).HasError())
			model.ID = types.StringValue("membership-1")
			assert.Equal(t, model, created)

			// Refresh discovers the current ID if the membership was recreated.
			client.member.Id = "membership-2"
			readResp := resource.ReadResponse{State: createResp.State}
			r.Read(ctx, resource.ReadRequest{State: createResp.State}, &readResp)
			require.False(t, readResp.Diagnostics.HasError(), "%v", readResp.Diagnostics)
			// Create already looked for an existing member.
			require.Len(t, client.gets, 2)
			assert.Equal(t, "group-1", client.gets[1].GetGroupId())
			assert.Equal(t, "member-1", client.gets[1].GetSubject().GetId())
			assert.Equal(t, principal, client.gets[1].GetSubject().GetPrincipal())
			var refreshed groupMembershipModel
			require.False(t, readResp.State.Get(ctx, &refreshed).HasError())
			model.ID = types.StringValue("membership-2")
			assert.Equal(t, model, refreshed)

			deleteResp := resource.DeleteResponse{State: readResp.State}
			r.Delete(ctx, resource.DeleteRequest{State: readResp.State}, &deleteResp)
			require.False(t, deleteResp.Diagnostics.HasError(), "%v", deleteResp.Diagnostics)
			require.Len(t, client.deletes, 1)
			assert.Equal(t, "membership-2", client.deletes[0].GetMembershipId())
		})
	}
}

func TestGroupMembershipResourceRead_MissingRemovesState(t *testing.T) {
	for name, err := range map[string]error{
		"empty response": nil,
		"not found":      connect.NewError(connect.CodeNotFound, errors.New("missing")),
	} {
		t.Run(name, func(t *testing.T) {
			client := &groupMembershipClient{getErr: err}
			r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
			state := groupMembershipTestState(t, groupMembershipModel{
				ID: types.StringValue("membership-1"), GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1"),
			})
			resp := resource.ReadResponse{State: state}
			r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.True(t, resp.State.Raw.IsNull())
			assert.Empty(t, client.deletes)
		})
	}
}

func TestGroupMembershipResourceRead_MissingID(t *testing.T) {
	client := &groupMembershipClient{member: &v1.GroupMembership{}}
	r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}

	state := groupMembershipTestState(t, groupMembershipModel{
		ID: types.StringValue("membership-1"), GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1"),
	})
	resp := resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	assert.True(t, state.Raw.Equal(resp.State.Raw), "the stored ID must be kept")

	// Nothing to fall back on, as right after an import.
	state = groupMembershipTestState(t, groupMembershipModel{GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1")})
	resp = resource.ReadResponse{State: state}
	r.Read(t.Context(), resource.ReadRequest{State: state}, &resp)
	require.True(t, resp.Diagnostics.HasError())
	assert.Equal(t, "Missing group membership ID", resp.Diagnostics[0].Summary())
}

func TestGroupMembershipResource_APIErrors(t *testing.T) {
	denied := connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	client := &groupMembershipClient{getErr: denied}
	r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
	state := groupMembershipTestState(t, groupMembershipModel{
		ID: types.StringValue("membership-1"), GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1"),
	})
	ctx := t.Context()

	createResp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	require.True(t, createResp.Diagnostics.HasError())
	assert.Equal(t, "Failed to read group membership before creation", createResp.Diagnostics[0].Summary())
	assert.Empty(t, client.creates)

	readResp := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &readResp)
	require.True(t, readResp.Diagnostics.HasError())
	assert.Equal(t, "Failed to read group membership", readResp.Diagnostics[0].Summary())
	assert.True(t, state.Raw.Equal(readResp.State.Raw), "a permission error must not remove state")

	deleteResp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)
	require.True(t, deleteResp.Diagnostics.HasError())
	assert.Equal(t, "Failed to read group membership before deletion", deleteResp.Diagnostics[0].Summary())
	assert.True(t, state.Raw.Equal(deleteResp.State.Raw))
	assert.Empty(t, client.deletes)

	client.getErr, client.createErr = nil, denied
	createResp = resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	require.True(t, createResp.Diagnostics.HasError())
	assert.Equal(t, "Failed to create group membership", createResp.Diagnostics[0].Summary())

	client.createErr = connect.NewError(connect.CodeAlreadyExists, errors.New("already a member"))
	createResp = resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	assert.True(t, createResp.Diagnostics.HasError(), "existing memberships must be imported, not silently adopted")

	client.member, client.deleteErr = &v1.GroupMembership{Id: "membership-1"}, denied
	deleteResp = resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)
	require.True(t, deleteResp.Diagnostics.HasError())
	assert.Equal(t, "Failed to delete group membership", deleteResp.Diagnostics[0].Summary())
	assert.True(t, state.Raw.Equal(deleteResp.State.Raw))

	client.deleteErr = connect.NewError(connect.CodeNotFound, errors.New("already deleted"))
	deleteResp = resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &deleteResp)
	assert.False(t, deleteResp.Diagnostics.HasError(), "%v", deleteResp.Diagnostics)
}

func TestGroupMembershipResource_CreateRejectsExistingMember(t *testing.T) {
	// An idempotent CreateMembership would hand the existing membership back.
	existing := &v1.GroupMembership{Id: "membership-1"}
	client := &groupMembershipClient{member: existing, created: existing}
	r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
	state := groupMembershipTestState(t, groupMembershipModel{GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1")})
	createResp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	require.True(t, createResp.Diagnostics.HasError())
	assert.Equal(t, "Group membership already exists", createResp.Diagnostics[0].Summary())
	assert.Empty(t, client.creates, "existing memberships must be imported, not silently adopted")
}

func TestGroupMembershipResource_CreateRecoversMissingID(t *testing.T) {
	client := &groupMembershipClient{created: &v1.GroupMembership{Id: "membership-1"}, createOmitsMember: true}
	r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
	state := groupMembershipTestState(t, groupMembershipModel{GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1")})
	createResp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	require.False(t, createResp.Diagnostics.HasError(), "an error would taint the new membership: %v", createResp.Diagnostics)
	var got groupMembershipModel
	require.False(t, createResp.State.Get(t.Context(), &got).HasError())
	assert.Equal(t, "membership-1", got.ID.ValueString())
}

func TestGroupMembershipResource_CreateWithoutIDPreservesLookupFields(t *testing.T) {
	client := &groupMembershipClient{}
	r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
	model := groupMembershipModel{GroupID: types.StringValue("group-1"), UserID: types.StringValue("user-1")}
	state := groupMembershipTestState(t, model)
	createResp := resource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(t.Context(), resource.CreateRequest{Plan: tfsdk.Plan(state)}, &createResp)
	require.True(t, createResp.Diagnostics.HasError())
	var got groupMembershipModel
	require.False(t, createResp.State.Get(t.Context(), &got).HasError())
	assert.Equal(t, model, got)

	client.member = &v1.GroupMembership{Id: "recovered-membership"}
	readResp := resource.ReadResponse{State: createResp.State}
	r.Read(t.Context(), resource.ReadRequest{State: createResp.State}, &readResp)
	require.False(t, readResp.Diagnostics.HasError(), "%v", readResp.Diagnostics)
	require.False(t, readResp.State.Get(t.Context(), &got).HasError())
	assert.Equal(t, "recovered-membership", got.ID.ValueString())
}

func TestGroupMembershipResource_DeleteResolvesCurrentID(t *testing.T) {
	for name, tc := range map[string]struct {
		storedID   string
		member     *v1.GroupMembership
		err        error
		wantError  bool
		wantDelete string
	}{
		"no stored ID":      {member: &v1.GroupMembership{Id: "membership-1"}, wantDelete: "membership-1"},
		"stale stored ID":   {storedID: "membership-0", member: &v1.GroupMembership{Id: "membership-1"}, wantDelete: "membership-1"},
		"absent":            {storedID: "membership-0"},
		"not found":         {storedID: "membership-0", err: connect.NewError(connect.CodeNotFound, errors.New("missing"))},
		"denied":            {storedID: "membership-0", err: connect.NewError(connect.CodePermissionDenied, errors.New("denied")), wantError: true},
		"API omits ID":      {storedID: "membership-1", member: &v1.GroupMembership{}, wantDelete: "membership-1"},
		"no ID from anyone": {member: &v1.GroupMembership{}, wantError: true},
	} {
		t.Run(name, func(t *testing.T) {
			client := &groupMembershipClient{member: tc.member, getErr: tc.err}
			r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
			state := groupMembershipTestState(t, groupMembershipModel{
				ID: stringValueOrNull(tc.storedID), GroupID: types.StringValue("group-1"), ServiceAccountID: types.StringValue("sa-1"),
			})
			resp := resource.DeleteResponse{State: state}
			r.Delete(t.Context(), resource.DeleteRequest{State: state}, &resp)
			assert.Equal(t, tc.wantError, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			require.Len(t, client.gets, 1)
			assert.Equal(t, "group-1", client.gets[0].GetGroupId())
			assert.Equal(t, "sa-1", client.gets[0].GetSubject().GetId())
			assert.Equal(t, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, client.gets[0].GetSubject().GetPrincipal())
			if tc.wantDelete == "" {
				assert.Empty(t, client.deletes)
				return
			}
			require.Len(t, client.deletes, 1)
			assert.Equal(t, tc.wantDelete, client.deletes[0].GetMembershipId())
		})
	}
}

func TestGroupMembershipResourceImportState(t *testing.T) {
	for _, memberType := range []string{"user", "service_account"} {
		t.Run(memberType, func(t *testing.T) {
			client := &groupMembershipClient{member: &v1.GroupMembership{Id: "membership-1"}}
			r := &groupMembershipResource{client: &sdk.Client{Services: sdk.ServiceClients{Group: client}}}
			resp := resource.ImportStateResponse{State: groupMembershipTestState(t, groupMembershipModel{})}
			r.ImportState(t.Context(), resource.ImportStateRequest{ID: "group-1/" + memberType + "/member-1"}, &resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

			readResp := resource.ReadResponse{State: resp.State}
			r.Read(t.Context(), resource.ReadRequest{State: resp.State}, &readResp)
			require.False(t, readResp.Diagnostics.HasError(), "%v", readResp.Diagnostics)
			var got groupMembershipModel
			require.False(t, readResp.State.Get(t.Context(), &got).HasError())
			assert.Equal(t, "membership-1", got.ID.ValueString())
			assert.Equal(t, "group-1", got.GroupID.ValueString())
			require.Len(t, client.gets, 1)
			assert.Equal(t, "member-1", client.gets[0].GetSubject().GetId())
			if memberType == "user" {
				assert.Equal(t, "member-1", got.UserID.ValueString())
				assert.True(t, got.ServiceAccountID.IsNull())
				assert.Equal(t, v1.Principal_PRINCIPAL_USER, client.gets[0].GetSubject().GetPrincipal())
			} else {
				assert.Equal(t, "member-1", got.ServiceAccountID.ValueString())
				assert.True(t, got.UserID.IsNull())
				assert.Equal(t, v1.Principal_PRINCIPAL_SERVICE_ACCOUNT, client.gets[0].GetSubject().GetPrincipal())
			}
			assert.Empty(t, client.creates, "import must not create a membership")
		})
	}

	for _, id := range []string{"", "membership-1", "group-1/member-1", "/user/member-1", "group-1/user/", "group-1/runner/member-1", "group-1//member-1", "group-1/user/member-1/extra"} {
		t.Run("invalid "+id, func(t *testing.T) {
			resp := resource.ImportStateResponse{State: groupMembershipTestState(t, groupMembershipModel{})}
			(&groupMembershipResource{}).ImportState(t.Context(), resource.ImportStateRequest{ID: id}, &resp)
			require.True(t, resp.Diagnostics.HasError())
			assert.Equal(t, "Invalid import ID", resp.Diagnostics[0].Summary())
		})
	}
}

func TestGroupMembershipResourceValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		userID           types.String
		serviceAccountID types.String
		wantError        bool
	}{
		"user":                  {userID: types.StringValue("user-1")},
		"service account":       {serviceAccountID: types.StringValue("sa-1")},
		"neither":               {wantError: true},
		"both":                  {userID: types.StringValue("user-1"), serviceAccountID: types.StringValue("sa-1"), wantError: true},
		"user from resource":    {userID: types.StringUnknown()},
		"account from resource": {serviceAccountID: types.StringUnknown()},
		"empty user":            {userID: types.StringValue(""), wantError: true},
		"empty service account": {serviceAccountID: types.StringValue(""), wantError: true},
	} {
		t.Run(name, func(t *testing.T) {
			r := &groupMembershipResource{}
			state := groupMembershipTestState(t, groupMembershipModel{
				GroupID: types.StringValue("group-1"), UserID: tc.userID, ServiceAccountID: tc.serviceAccountID,
			})
			config := tfsdk.Config(state)
			resp := resource.ValidateConfigResponse{}
			for _, v := range r.ConfigValidators(t.Context()) {
				v.ValidateResource(t.Context(), resource.ValidateConfigRequest{Config: config}, &resp)
			}
			var schemaResp resource.SchemaResponse
			r.Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
			for attrName, value := range map[string]types.String{"user_id": tc.userID, "service_account_id": tc.serviceAccountID} {
				attr, ok := schemaResp.Schema.Attributes[attrName].(schema.StringAttribute)
				require.True(t, ok)
				for _, v := range attr.Validators {
					attrResp := validator.StringResponse{}
					v.ValidateString(t.Context(), validator.StringRequest{Path: path.Root(attrName), Config: config, ConfigValue: value}, &attrResp)
					resp.Diagnostics.Append(attrResp.Diagnostics...)
				}
			}
			assert.Equal(t, tc.wantError, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
		})
	}
}

func TestGroupMembershipResource_MemberAndGroupChangesRequireReplacement(t *testing.T) {
	var schemaResp resource.SchemaResponse
	NewGroupMembershipResource().Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)
	raw := nonNullRawValue(t.Context(), t, schemaResp.Schema)
	for _, name := range []string{"group_id", "user_id", "service_account_id"} {
		t.Run(name, func(t *testing.T) {
			attr, ok := schemaResp.Schema.Attributes[name].(schema.StringAttribute)
			require.True(t, ok)
			for _, newValue := range []types.String{types.StringValue("new-id"), types.StringNull()} {
				resp := planmodifier.StringResponse{}
				for _, modifier := range attr.PlanModifiers {
					modifier.PlanModifyString(t.Context(), planmodifier.StringRequest{
						State: tfsdk.State{Raw: raw, Schema: schemaResp.Schema}, Plan: tfsdk.Plan{Raw: raw, Schema: schemaResp.Schema},
						StateValue: types.StringValue("old-id"), PlanValue: newValue, ConfigValue: newValue,
					}, &resp)
				}
				assert.True(t, resp.RequiresReplace, "%s changed to %v", name, newValue)
			}
		})
	}
}

func groupMembershipTestState(t *testing.T, model groupMembershipModel) tfsdk.State {
	t.Helper()
	var resp resource.SchemaResponse
	NewGroupMembershipResource().Schema(t.Context(), resource.SchemaRequest{}, &resp)
	state := tfsdk.State{Schema: resp.Schema}
	diags := state.Set(t.Context(), &model)
	require.False(t, diags.HasError(), "%v", diags)
	return state
}

type groupMembershipClient struct {
	v1connect.GroupServiceClient
	// member is what GetMembership reports; nil means the subject is not a member.
	member *v1.GroupMembership
	// created becomes member on CreateMembership. createOmitsMember leaves it
	// out of the create response.
	created           *v1.GroupMembership
	createOmitsMember bool
	createErr         error
	getErr            error
	deleteErr         error
	creates           []*v1.CreateMembershipRequest
	gets              []*v1.GetMembershipRequest
	deletes           []*v1.DeleteMembershipRequest
}

func (c *groupMembershipClient) CreateMembership(_ context.Context, req *connect.Request[v1.CreateMembershipRequest]) (*connect.Response[v1.CreateMembershipResponse], error) {
	c.creates = append(c.creates, req.Msg)
	if c.createErr != nil {
		return nil, c.createErr
	}
	c.member = c.created
	if c.createOmitsMember {
		return connect.NewResponse(&v1.CreateMembershipResponse{}), nil
	}
	return connect.NewResponse(&v1.CreateMembershipResponse{Member: c.created}), nil
}

func (c *groupMembershipClient) GetMembership(_ context.Context, req *connect.Request[v1.GetMembershipRequest]) (*connect.Response[v1.GetMembershipResponse], error) {
	c.gets = append(c.gets, req.Msg)
	if c.getErr != nil {
		return nil, c.getErr
	}
	return connect.NewResponse(&v1.GetMembershipResponse{Member: c.member}), nil
}

func (c *groupMembershipClient) DeleteMembership(_ context.Context, req *connect.Request[v1.DeleteMembershipRequest]) (*connect.Response[v1.DeleteMembershipResponse], error) {
	c.deletes = append(c.deletes, req.Msg)
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return connect.NewResponse(&v1.DeleteMembershipResponse{}), nil
}
