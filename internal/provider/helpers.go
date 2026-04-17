package provider

import (
	"errors"
	"fmt"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringValueOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
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

func stringValueOrPriorExplicitEmpty(current string, prior types.String) types.String {
	if current != "" {
		return types.StringValue(current)
	}
	if !prior.IsNull() && !prior.IsUnknown() && prior.ValueString() == "" {
		return types.StringValue("")
	}
	return types.StringNull()
}

func stringListValue(values []string) types.List {
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func timeValueOrNull(value time.Time) types.String {
	if value.IsZero() {
		return types.StringNull()
	}
	return types.StringValue(value.Format(time.RFC3339Nano))
}

func mapUpdateWindowValues(window gitpod.UpdateWindow) (types.Int64, types.Int64, bool) {
	if window.JSON.RawJSON() == "" {
		return types.Int64Null(), types.Int64Null(), false
	}

	endHour := types.Int64Null()
	if !window.JSON.EndHour.IsMissing() {
		endHour = types.Int64Value(window.EndHour)
	}

	return types.Int64Value(window.StartHour), endHour, true
}

// isAPINotFound reports whether err is a Gitpod API error with status 404.
func isAPINotFound(err error) bool {
	var apiErr *gitpod.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

// clientFromProviderData extracts the *gitpod.Client from the ProviderData
// passed to a resource or data source Configure call. Returns false (without
// adding a diagnostic) when ProviderData is nil — the framework calls
// Configure before provider configuration has completed. Returns false with a
// diagnostic when ProviderData is of an unexpected type.
func clientFromProviderData(providerData any, diagnostics *diag.Diagnostics) (*gitpod.Client, bool) {
	if providerData == nil {
		return nil, false
	}
	client, ok := providerData.(*gitpod.Client)
	if !ok {
		diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *gitpod.Client, got %T", providerData))
		return nil, false
	}
	return client, true
}

var runnerStatusAttrTypes = map[string]attr.Type{
	"phase":   types.StringType,
	"message": types.StringType,
	"version": types.StringType,
	"region":  types.StringType,
}

// runnerStatusObjectValue converts a gitpod.RunnerStatus to a types.Object
// using the shared attribute-type map.
func runnerStatusObjectValue(status gitpod.RunnerStatus) types.Object {
	obj, _ := types.ObjectValue(runnerStatusAttrTypes, map[string]attr.Value{
		"phase":   types.StringValue(string(status.Phase)),
		"message": types.StringValue(status.Message),
		"version": types.StringValue(status.Version),
		"region":  types.StringValue(status.Region),
	})
	return obj
}
