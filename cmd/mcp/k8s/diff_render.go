// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package k8s

import (
	"fmt"
	"strings"

	"github.com/wI2L/jsondiff"
	"sigs.k8s.io/yaml"
)

// diffState is a renderer-level manifest change state.
type diffState string

const (
	diffStateCreate    diffState = "create"
	diffStateUpdate    diffState = "update"
	diffStateRecreate  diffState = "recreate"
	diffStatePatched   diffState = "patched"
	diffStateUnchanged diffState = "unchanged"
	diffStateSkipped   diffState = "skipped"
	diffStateDelete    diffState = "delete"
	diffStateError     diffState = "error"
)

// diffObjectResult is the pure renderer model for one manifest or inventory object.
type diffObjectResult struct {
	Subject string
	State   diffState
	Detail  string
	Hint    *DiffOwnerRef
	Patch   jsondiff.Patch
}

// diffResult is the complete pure renderer model for a diff request.
type diffResult struct {
	Owner         *DiffOwnerRef
	FieldManager  string
	PruneEnabled  bool
	FutureSuspend bool
	LiveSuspend   bool
	Warnings      []string
	Objects       []diffObjectResult
	PruneObjects  []diffObjectResult
	PruneStatus   string
}

// renderDiff renders a deterministic human-readable diff and summary.
func renderDiff(result diffResult) (string, error) {
	var builder strings.Builder
	if result.Owner != nil {
		fmt.Fprintf(&builder, "Diff for %s/%s/%s (field manager: %s, prune: %s)\n",
			result.Owner.Kind, result.Owner.Namespace, result.Owner.Name,
			result.FieldManager, enabledDisabled(result.PruneEnabled))
	} else {
		fmt.Fprintf(&builder, "Diff for Kubernetes manifest (field manager: %s, prune: disabled)\n", result.FieldManager)
	}

	if result.FutureSuspend {
		builder.WriteString("suspended: changes are not applied until resumed\n")
	} else if result.LiveSuspend {
		builder.WriteString("currently suspended; the proposed definition resumes it\n")
	}
	for _, warning := range result.Warnings {
		builder.WriteString(warning)
		builder.WriteString("\n")
	}

	if noChanges(result) {
		builder.WriteString("\nNo changes detected\n")
		return builder.String(), nil
	}

	builder.WriteString("\n")
	for _, object := range result.Objects {
		if err := renderDiffObject(&builder, object); err != nil {
			return "", err
		}
	}

	if result.PruneStatus != "" {
		if len(result.Objects) > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(result.PruneStatus)
		builder.WriteString("\n")
	}
	if len(result.PruneObjects) > 0 {
		builder.WriteString("\nNot in the manifest (pruned if the manifest is complete):\n")
		for _, object := range result.PruneObjects {
			if err := renderDiffObject(&builder, object); err != nil {
				return "", err
			}
		}
	}

	counts := make(map[diffState]int)
	for _, object := range result.Objects {
		counts[object.State]++
	}
	for _, object := range result.PruneObjects {
		counts[object.State]++
	}
	parts := make([]string, 0, 7)
	for _, state := range []diffState{
		diffStateCreate, diffStateUpdate, diffStateRecreate, diffStateUnchanged,
		diffStateSkipped, diffStateDelete, diffStateError,
	} {
		if count := counts[state]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, state))
		}
	}
	if len(parts) > 0 {
		builder.WriteString("\nSummary: ")
		builder.WriteString(strings.Join(parts, ", "))
		builder.WriteString("\n")
	}
	return builder.String(), nil
}

// renderDiffObject writes one object state and optional JSON patch in YAML form.
func renderDiffObject(builder *strings.Builder, object diffObjectResult) error {
	fmt.Fprintf(builder, "%s %s", object.Subject, object.State)
	if object.Detail != "" {
		if object.State == diffStateError {
			fmt.Fprintf(builder, ": %s", object.Detail)
		} else {
			fmt.Fprintf(builder, " (%s)", object.Detail)
		}
	}
	if object.Hint != nil {
		fmt.Fprintf(builder, " (currently managed by %s/%s/%s)", object.Hint.Kind, object.Hint.Namespace, object.Hint.Name)
	}
	builder.WriteString("\n")
	if len(object.Patch) > 0 {
		patch, err := yaml.Marshal(object.Patch)
		if err != nil {
			return fmt.Errorf("marshalling JSON patch: %w", err)
		}
		builder.Write(patch)
	}
	return nil
}

// noChanges reports whether all manifest objects are unchanged and there are no prune candidates.
func noChanges(result diffResult) bool {
	if len(result.PruneObjects) > 0 || result.PruneStatus != "" || len(result.Objects) == 0 {
		return false
	}
	for _, object := range result.Objects {
		if object.State != diffStateUnchanged {
			return false
		}
	}
	return true
}

// enabledDisabled renders a boolean policy as enabled or disabled.
func enabledDisabled(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
