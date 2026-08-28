// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package toolbox

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/controlplaneio-fluxcd/flux-operator/cmd/mcp/k8s"
)

const (
	// ToolGetKubernetesEvents is the name of the get_kubernetes_events tool.
	ToolGetKubernetesEvents = "get_kubernetes_events"
)

func init() {
	systemTools[ToolGetKubernetesEvents] = systemTool{
		readOnly:  true,
		inCluster: true,
	}
}

// getKubernetesEventsInput defines the input parameters for retrieving Kubernetes events.
type getKubernetesEventsInput struct {
	APIVersion string  `json:"apiVersion,omitempty" jsonschema:"Exact apiVersion of the involved object."`
	Kind       string  `json:"kind,omitempty"       jsonschema:"Exact kind of the involved object."`
	Name       string  `json:"name,omitempty"       jsonschema:"Exact name of the involved object."`
	Namespace  string  `json:"namespace,omitempty"  jsonschema:"Namespace of the involved objects; omit to list across all namespaces."`
	Type       string  `json:"type,omitempty"       jsonschema:"Event type: Normal or Warning."`
	Since      string  `json:"since,omitempty"      jsonschema:"Only events newer than this Go duration, for example 10m or 1h."`
	Grep       string  `json:"grep,omitempty"       jsonschema:"Case-insensitive RE2 expression matched against reason, message and the object rendered as Kind/namespace/name."`
	Limit      float64 `json:"limit,omitempty"      jsonschema:"Maximum number of events to return. Defaults to 100."`
}

// HandleGetKubernetesEvents is the handler function for the get_kubernetes_events tool.
func (m *Manager) HandleGetKubernetesEvents(ctx context.Context, request *mcp.CallToolRequest, input getKubernetesEventsInput) (*mcp.CallToolResult, any, error) {
	if err := CheckScopes(ctx, ToolGetKubernetesEvents, m.readOnly); err != nil {
		return NewToolResultError(err.Error())
	}

	if input.Type != "" && input.Type != corev1.EventTypeNormal && input.Type != corev1.EventTypeWarning {
		return NewToolResultError("type must be Normal or Warning")
	}

	var since *time.Duration
	if input.Since != "" {
		parsed, err := time.ParseDuration(input.Since)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Invalid since: %v", err))
		}
		if parsed <= 0 {
			return NewToolResultError("Invalid since: must be a positive duration")
		}
		since = &parsed
	}

	var grep *regexp.Regexp
	if input.Grep != "" {
		var err error
		grep, err = regexp.Compile("(?i)" + input.Grep)
		if err != nil {
			return NewToolResultError(fmt.Sprintf("Invalid grep: %v", err))
		}
	}

	limit := int(input.Limit)
	if limit <= 0 {
		limit = 100
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	kubeClient, err := m.kubeClient.GetClient(ctx)
	if err != nil {
		return NewToolResultErrorFromErr("Failed to get Kubernetes client", err)
	}

	result, err := kubeClient.ListEvents(ctx, k8s.ListEventsOptions{
		Namespace:  input.Namespace,
		APIVersion: input.APIVersion,
		Kind:       input.Kind,
		Name:       input.Name,
		Type:       input.Type,
		Since:      since,
		Grep:       grep,
		Limit:      limit,
	})
	if err != nil {
		return NewToolResultError(err.Error())
	}
	if result.Total == 0 && !result.Truncated {
		return NewToolResultText("No events found")
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return NewToolResultErrorFromErr("Failed marshalling data", err)
	}

	return NewToolResultText(string(data))
}
