// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Flux resource kinds and their metadata (ordered by group for display)
export const fluxCRDs = [
  // Appliers
  {
    kind: 'FluxInstance',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'fluxinstance',
    group: 'Appliers',
    docUrl: 'https://fluxcd.control-plane.io/operator/fluxinstance/',
  },
  {
    kind: 'ResourceSet',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'rset',
    group: 'Appliers',
    docUrl: 'https://fluxcd.control-plane.io/operator/resourceset/',
  },
  {
    kind: 'Kustomization',
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    alias: 'ks',
    group: 'Appliers',
    docUrl: 'https://toolkit.fluxcd.io/components/kustomize/kustomizations/',
  },
  {
    kind: 'HelmRelease',
    apiVersion: 'helm.toolkit.fluxcd.io/v2',
    alias: 'hr',
    group: 'Appliers',
    docUrl: 'https://toolkit.fluxcd.io/components/helm/helmreleases/',
  },
  // Sources
  {
    kind: "ArtifactGenerator",
    apiVersion: "source.extensions.fluxcd.io/v1beta1",
    alias: "ag",
    group: 'Sources',
    docUrl: "https://fluxcd.io/flux/components/source/artifactgenerators/",
  },
  {
    kind: 'Bucket',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'bucket',
    group: 'Sources',
    docUrl: 'https://fluxcd.io/docs/components/source/buckets/',
  },
  {
    kind: "ExternalArtifact",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "ea",
    group: 'Sources',
    docUrl: "https://fluxcd.io/flux/components/source/externalartifacts/",
  },
  {
    kind: 'GitRepository',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'gitrepo',
    group: 'Sources',
    docUrl: 'https://fluxcd.io/docs/components/source/gitrepositories/',
  },
  {
    kind: "HelmChart",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "helmchart",
    group: 'Sources',
    docUrl: "https://toolkit.fluxcd.io/components/source/helmcharts/",
  },
  {
    kind: "HelmRepository",
    apiVersion: "source.toolkit.fluxcd.io/v1",
    alias: "helmrepo",
    group: 'Sources',
    docUrl: "https://fluxcd.io/docs/components/source/helmrepositories/",
  },
  {
    kind: 'OCIRepository',
    apiVersion: 'source.toolkit.fluxcd.io/v1',
    alias: 'ocirepo',
    group: 'Sources',
    docUrl: 'https://fluxcd.io/docs/components/source/ocirepositories/',
  },
  {
    kind: 'ResourceSetInputProvider',
    apiVersion: 'fluxcd.controlplane.io/v1',
    alias: 'rsip',
    group: 'Sources',
    docUrl: 'https://fluxcd.control-plane.io/operator/resourcesetinputprovider/',
  },
  // Notifications
  {
    kind: "Alert",
    apiVersion: "notification.toolkit.fluxcd.io/v1beta3",
    alias: "alert",
    group: 'Notifications',
    docUrl: "https://fluxcd.io/docs/components/notification/alerts/",
  },
  {
    kind: "Provider",
    apiVersion: "notification.toolkit.fluxcd.io/v1beta3",
    alias: "provider",
    group: 'Notifications',
    docUrl: "https://fluxcd.io/docs/components/notification/providers/",
  },
  {
    kind: "Receiver",
    apiVersion: "notification.toolkit.fluxcd.io/v1",
    alias: "receiver",
    group: 'Notifications',
    docUrl: "https://fluxcd.io/docs/components/notification/receivers/",
  },
  // Image Automation
  {
    kind: "ImagePolicy",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgpol",
    group: 'Image Automation',
    docUrl: "https://fluxcd.io/docs/components/image/imagepolicies/",
  },
  {
    kind: "ImageRepository",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgrepo",
    group: 'Image Automation',
    docUrl: "https://fluxcd.io/docs/components/image/imagerepositories/",
  },
  {
    kind: "ImageUpdateAutomation",
    apiVersion: "image.toolkit.fluxcd.io/v1",
    alias: "imgauto",
    group: 'Image Automation',
    docUrl: "https://fluxcd.io/docs/components/image/imageupdateautomations/",
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
