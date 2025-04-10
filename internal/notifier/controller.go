// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package notifier

import "fmt"

const (
	// Controller is the name of notification-controller.
	Controller = "notification-controller"
)

// Address returns the address of the notification-controller for
// the given namespace and clusterDomain for sending events.
func Address(namespace, clusterDomain string) string {
	return fmt.Sprintf("http://%s.%s.svc.%s./", Controller, namespace, clusterDomain)
}
