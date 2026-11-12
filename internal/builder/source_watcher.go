// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package builder

import (
	"fmt"

	fluxcdv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
)

// ValidateAndApplySourceWatcherConfig validates the source-watcher
// configuration and enables the generation of the namespace manager RBAC.
// The source-watcher component must be included in the installation when
// the source-watcher configuration is present.
func (o *Options) ValidateAndApplySourceWatcherConfig(sourceWatcher *fluxcdv1.SourceWatcher) error {
	if sourceWatcher == nil {
		return nil
	}
	if !o.HasSourceWatcher() {
		return fmt.Errorf(".spec.cluster.sourceWatcher requires the source-watcher component to be enabled")
	}

	o.SourceWatcherNamespaceManager = sourceWatcher.NamespaceManager
	return nil
}
