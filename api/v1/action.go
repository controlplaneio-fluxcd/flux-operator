// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

// Action describes an observable stage of the reconcile loop. It is sent
// as the action of the events forwarded to notification-controller.
type Action string

// String returns the string representation of the Action.
func (a Action) String() string {
	return string(a)
}

const (
	// ActionReconcile denotes the overall outcome of the reconcile loop.
	ActionReconcile Action = "Reconcile"

	// ActionFetchDistribution fetches the Flux distribution artifact
	// referenced by a FluxInstance.
	ActionFetchDistribution Action = "FetchDistribution"

	// ActionBuild builds the Kubernetes manifests from the FluxInstance
	// distribution or the ResourceSet templates.
	ActionBuild Action = "Build"

	// ActionCheckDependencies verifies that every resource referenced by
	// a ResourceSet spec.dependsOn is ready before reconciliation proceeds.
	ActionCheckDependencies Action = "CheckDependencies"

	// ActionApply reconciles the built manifests onto the cluster using
	// server-side apply.
	ActionApply Action = "Apply"

	// ActionSchedule evaluates the ResourceSetInputProvider schedule.
	ActionSchedule Action = "Schedule"

	// ActionValidate validates the ResourceSetInputProvider spec and
	// default values.
	ActionValidate Action = "Validate"

	// ActionExportInputs exports the inputs computed by a
	// ResourceSetInputProvider.
	ActionExportInputs Action = "ExportInputs"
)
