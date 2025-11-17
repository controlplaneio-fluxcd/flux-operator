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
