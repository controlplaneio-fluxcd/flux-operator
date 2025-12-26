// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Auto-refresh polling interval in milliseconds
export const POLL_INTERVAL_MS = 30000

// Flux resource kinds and their metadata (ordered by group for display)
export const fluxCRDs = [
  // Appliers
  {
    kind: 'FluxInstance',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'fluxinstance',
    group: 'Appliers',
    docUrl: 'https://fluxoperator.dev/docs/crd/fluxinstance/',
  },
  {
    kind: 'ResourceSet',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'rset',
    group: 'Appliers',
    docUrl: 'https://fluxoperator.dev/docs/crd/resourceset/',
  },
  {
    kind: 'Kustomization',
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    alias: 'ks',
    group: 'Appliers',
    docUrl: 'https://fluxoperator.dev/docs/crd/kustomization/',
  },
  {
    kind: 'HelmRelease',
    apiVersion: 'helm.toolkit.fluxcd.io/v2',
    alias: 'hr',
    group: 'Appliers',
    docUrl: 'https://fluxoperator.dev/docs/crd/helmrelease/',
  },
  // Sources
  {
    kind: "ArtifactGenerator",
    apiVersion: "source.extensions.fluxcd.io/v1beta1",
    alias: "ag",
    group: 'Sources',
    docUrl: "https://fluxoperator.dev/docs/crd/artifactgenerator/",
  },
  {
    kind: 'Bucket',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'bucket',
    group: 'Sources',
    docUrl: 'https://fluxoperator.dev/docs/crd/bucket/',
  },
  {
    kind: "ExternalArtifact",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "ea",
    group: 'Sources',
    docUrl: "https://fluxoperator.dev/docs/crd/externalartifact/",
  },
  {
    kind: 'GitRepository',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'gitrepo',
    group: 'Sources',
    docUrl: 'https://fluxoperator.dev/docs/crd/gitrepository/',
  },
  {
    kind: "HelmChart",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "helmchart",
    group: 'Sources',
    docUrl: "https://fluxoperator.dev/docs/crd/helmchart/",
  },
  {
    kind: "HelmRepository",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "helmrepo",
    group: 'Sources',
    docUrl: "https://fluxoperator.dev/docs/crd/helmrepository/",
  },
  {
    kind: 'OCIRepository',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'ocirepo',
    group: 'Sources',
    docUrl: 'https://fluxoperator.dev/docs/crd/ocirepository/',
  },
  {
    kind: 'ResourceSetInputProvider',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'rsip',
    group: 'Sources',
    docUrl: 'https://fluxoperator.dev/docs/crd/resourcesetinputprovider/',
  },
  // Notifications
  {
    kind: "Alert",
    apiVersion: "notification.toolkit.fluxcd.io/v1beta3",
    alias: "alert",
    group: 'Notifications',
    docUrl: "https://fluxoperator.dev/docs/crd/alert/",
  },
  {
    kind: "Provider",
    apiVersion: "notification.toolkit.fluxcd.io/v1beta3",
    alias: "provider",
    group: 'Notifications',
    docUrl: "https://fluxoperator.dev/docs/crd/provider/",
  },
  {
    kind: "Receiver",
    apiVersion: "notification.toolkit.fluxcd.io/v1",
    alias: "receiver",
    group: 'Notifications',
    docUrl: "https://fluxoperator.dev/docs/crd/receiver/",
  },
  // Image Automation
  {
    kind: "ImagePolicy",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgpol",
    group: 'Image Automation',
    docUrl: "https://fluxoperator.dev/docs/crd/imagepolicy/",
  },
  {
    kind: "ImageRepository",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgrepo",
    group: 'Image Automation',
    docUrl: "https://fluxoperator.dev/docs/crd/imagerepository/",
  },
  {
    kind: "ImageUpdateAutomation",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgauto",
    group: 'Image Automation',
    docUrl: "https://fluxoperator.dev/docs/crd/imageupdateautomation/",
  },
]

// Flux resource kinds for dropdown (derived from fluxCRDs)
export const fluxKinds = fluxCRDs.map(crd => crd.kind)

// Event severity options (based on Kubernetes event Type field)
export const eventSeverities = ['Normal', 'Warning']

// Resource status options (based on Kubernetes condition status)
export const resourceStatuses = ['Ready', 'Failed', 'Progressing', 'Suspended', 'Unknown']

// Kubernetes workload kinds
export const workloadKinds = [
  'DaemonSet',
  'Deployment',
  'StatefulSet'
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

/**
 * Get the short alias for a given resource kind
 * @param {string} kind - The resource kind
 * @returns {string} - The alias (e.g., 'gitrepo' for 'GitRepository')
 */
export function getKindAlias(kind) {
  const crd = fluxCRDs.find(c => c.kind === kind)
  return crd?.alias || kind.toLowerCase()
}
