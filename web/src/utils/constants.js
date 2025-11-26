// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Flux resource kinds for dropdown (ordered by API group)
export const fluxKinds = [
  // Flux Operator (fluxcd.controlplane.io)
  'FluxInstance',
  'ResourceSet',
  'ResourceSetInputProvider',
  // Appliers
  'Kustomization',
  'HelmRelease',
  // Sources (source.toolkit.fluxcd.io)
  'GitRepository',
  'OCIRepository',
  'HelmRepository',
  'HelmChart',
  'Bucket',
  // Extensions (source.extensions.fluxcd.io)
  'ArtifactGenerator',
  'ExternalArtifact',
  // Image Automation (image.toolkit.fluxcd.io)
  'ImageRepository',
  'ImagePolicy',
  'ImageUpdateAutomation',
  // Notifications (notification.toolkit.fluxcd.io)
  'Alert',
  'Provider',
  'Receiver'
]

// Event severity options (based on Kubernetes event Type field)
export const eventSeverities = ['Normal', 'Warning']

// Resource status options (based on Kubernetes condition status)
export const resourceStatuses = ['Ready', 'Failed', 'Progressing', 'Suspended', 'Unknown']

// Kubernetes workload kinds
export const workloadKinds = [
  'Deployment',
  'StatefulSet',
  'DaemonSet'
]

// Map resource kind to controller name
const kindToControllerMap = {
  'FluxInstance': 'flux-operator',
  'ResourceSet': 'flux-operator',
  'ResourceSetInputProvider': 'flux-operator',
  'Kustomization': 'kustomize-controller',
  'HelmRelease': 'helm-controller',
  'GitRepository': 'source-controller',
  'OCIRepository': 'source-controller',
  'Bucket': 'source-controller',
  'HelmRepository': 'source-controller',
  'HelmChart': 'source-controller',
  'ExternalArtifact': 'source-watcher',
  'ArtifactGenerator': 'source-watcher',
  'Alert': 'notification-controller',
  'Provider': 'notification-controller',
  'Receiver': 'notification-controller',
  'ImageRepository': 'image-reflector-controller',
  'ImagePolicy': 'image-reflector-controller',
  'ImageUpdateAutomation': 'image-automation-controller'
}

/**
 * Get the controller name for a given resource kind
 * @param {string} kind - The resource kind
 * @returns {string} - The controller name
 */
export function getControllerName(kind) {
  return kindToControllerMap[kind] || 'unknown'
}

/**
 * Check if a resource kind should have an inventory
 * @param {string} kind - Resource kind
 * @returns {boolean} True if kind has inventory
 */
export function isKindWithInventory(kind) {
  return kind === 'Kustomization' || kind === 'HelmRelease' || kind === 'ArtifactGenerator' ||
      kind === 'FluxInstance' || kind === 'ResourceSet'
}
