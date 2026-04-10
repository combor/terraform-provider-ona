package provider

import (
	"context"
	"time"

	gitpod "github.com/gitpod-io/gitpod-sdk-go"
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
