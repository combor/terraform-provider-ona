package provider

import (
	"testing"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapGroupsToDataSourceModel_MapsAndSortsGroups(t *testing.T) {
	got := mapGroupsToDataSourceModel(nil, []gitpod.Group{
		{
			ID:             "group-b",
			Name:           "Backend",
			Description:    "Backend team",
			OrganizationID: "org-1",
			MemberCount:    3,
			DirectShare:    false,
			SystemManaged:  false,
			CreatedAt:      time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID:             "group-a",
			Name:           "Frontend",
			Description:    "Frontend team",
			OrganizationID: "org-1",
			MemberCount:    5,
			DirectShare:    true,
			SystemManaged:  true,
			CreatedAt:      time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
			UpdatedAt:      time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC),
		},
	})

	assert.Empty(t, got.Filters)
	require.Len(t, got.Groups, 2)

	assert.Equal(t, "group-a", got.Groups[0].ID.ValueString())
	assert.Equal(t, "Frontend", got.Groups[0].Name.ValueString())
	assert.Equal(t, "Frontend team", got.Groups[0].Description.ValueString())
	assert.Equal(t, "org-1", got.Groups[0].OrganizationID.ValueString())
	assert.Equal(t, int64(5), got.Groups[0].MemberCount.ValueInt64())
	assert.True(t, got.Groups[0].DirectShare.ValueBool())
	assert.True(t, got.Groups[0].SystemManaged.ValueBool())
	assert.Equal(t, "2026-01-02T00:00:00Z", got.Groups[0].CreatedAt.ValueString())
	assert.Equal(t, "2026-02-02T00:00:00Z", got.Groups[0].UpdatedAt.ValueString())

	assert.Equal(t, "group-b", got.Groups[1].ID.ValueString())
	assert.Equal(t, "Backend", got.Groups[1].Name.ValueString())
}

func TestMapGroupsToDataSourceModel_EmptyList(t *testing.T) {
	got := mapGroupsToDataSourceModel(nil, []gitpod.Group{})
	assert.Empty(t, got.Filters)
	assert.Empty(t, got.Groups)
}

func TestMapGroupsToDataSourceModel_EmptyOptionalFields(t *testing.T) {
	got := mapGroupsToDataSourceModel(nil, []gitpod.Group{
		{
			ID: "group-1",
		},
	})

	require.Len(t, got.Groups, 1)
	assert.Equal(t, "group-1", got.Groups[0].ID.ValueString())
	assert.True(t, got.Groups[0].Name.IsNull())
	assert.True(t, got.Groups[0].Description.IsNull())
	assert.True(t, got.Groups[0].OrganizationID.IsNull())
	assert.True(t, got.Groups[0].CreatedAt.IsNull())
	assert.True(t, got.Groups[0].UpdatedAt.IsNull())
}

func TestMatchesGroupFilters_NoFilters(t *testing.T) {
	assert.True(t, matchesGroupFilters(gitpod.Group{Name: "anything"}, nil))
}

func TestMatchesGroupFilters_NameMatch(t *testing.T) {
	filters := []groupsFilterModel{
		{
			Name:   types.StringValue("name"),
			Values: []types.String{types.StringValue("Engineering"), types.StringValue("Backend")},
		},
	}

	assert.True(t, matchesGroupFilters(gitpod.Group{Name: "Engineering"}, filters))
	assert.True(t, matchesGroupFilters(gitpod.Group{Name: "Backend"}, filters))
	assert.False(t, matchesGroupFilters(gitpod.Group{Name: "Frontend"}, filters))
}

func TestMatchesGroupFilters_UnsupportedFilter(t *testing.T) {
	filters := []groupsFilterModel{
		{
			Name:   types.StringValue("unknown"),
			Values: []types.String{types.StringValue("value")},
		},
	}

	assert.False(t, matchesGroupFilters(gitpod.Group{Name: "anything"}, filters))
}
