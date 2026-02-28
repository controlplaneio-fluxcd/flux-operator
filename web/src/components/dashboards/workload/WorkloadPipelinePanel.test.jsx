// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/preact'
import { WorkloadPipelinePanel } from './WorkloadPipelinePanel'

const baseReconciler = {
  apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
  kind: 'Kustomization',
  metadata: {
    name: 'apps',
    namespace: 'flux-system'
  },
  status: {
    reconcilerRef: {
      status: 'Ready'
    },
    sourceRef: {
      kind: 'GitRepository',
      name: 'flux-system',
      namespace: 'flux-system',
      status: 'Ready',
      url: 'https://github.com/example/repo'
    },
    lastAttemptedRevision: 'main@sha1:abc123'
  }
}

const basePods = [
  { name: 'nginx-abc-123', status: 'Running' },
  { name: 'nginx-abc-456', status: 'Running' }
]

describe('WorkloadPipelinePanel', () => {
  it('should not render when reconciler is null', () => {
    const { container } = render(
      <WorkloadPipelinePanel
        reconciler={null}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )
    expect(container.innerHTML).toBe('')
  })

  it('should render all 4 pipeline nodes', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    expect(screen.getByTestId('workload-pipeline-panel')).toBeInTheDocument()

    // All nodes rendered (doubled for desktop + mobile)
    const nodes = screen.getAllByTestId('pipeline-node')
    expect(nodes).toHaveLength(8) // 4 nodes x 2 layouts

    // Source node (CSS uppercase class transforms visually, DOM text is original case)
    expect(screen.getAllByText('GitRepository').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('flux-system').length).toBeGreaterThanOrEqual(1)

    // Reconciler node
    expect(screen.getAllByText('Kustomization').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('apps').length).toBeGreaterThanOrEqual(1)

    // Workload node
    expect(screen.getAllByText('Deployment').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('nginx').length).toBeGreaterThanOrEqual(1)

    // Pods node — readiness as name, phase summary as subtext
    expect(screen.getAllByText('Pods').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('2/2 ready').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('2 running').length).toBeGreaterThanOrEqual(1)
  })

  it('should show source URL as subtext', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    expect(screen.getAllByText('https://github.com/example/repo').length).toBeGreaterThanOrEqual(1)
  })

  it('should show reconciler revision as subtext', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    expect(screen.getAllByText('main@sha1:abc123').length).toBeGreaterThanOrEqual(1)
  })

  it('should render clickable links for source and reconciler', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    const links = screen.getAllByTestId('pipeline-link')
    // 2 clickable nodes (source + reconciler) x 2 layouts = 4
    expect(links).toHaveLength(4)

    // Check source link href
    const sourceLinks = links.filter(l => l.getAttribute('href').includes('GitRepository'))
    expect(sourceLinks.length).toBeGreaterThanOrEqual(1)
    expect(sourceLinks[0].getAttribute('href')).toBe('/resource/GitRepository/flux-system/flux-system')

    // Check reconciler link href
    const reconcilerLinks = links.filter(l => l.getAttribute('href').includes('Kustomization'))
    expect(reconcilerLinks.length).toBeGreaterThanOrEqual(1)
    expect(reconcilerLinks[0].getAttribute('href')).toBe('/resource/Kustomization/flux-system/apps')
  })

  it('should handle missing source (no sourceRef)', () => {
    const reconcilerNoSource = {
      ...baseReconciler,
      status: {
        ...baseReconciler.status,
        sourceRef: undefined
      }
    }

    render(
      <WorkloadPipelinePanel
        reconciler={reconcilerNoSource}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    // Should have 3 nodes instead of 4 (no source) x 2 layouts = 6
    const nodes = screen.getAllByTestId('pipeline-node')
    expect(nodes).toHaveLength(6)

    // Source kind should not appear
    expect(screen.queryByText('GitRepository')).not.toBeInTheDocument()

    // Reconciler, workload, and pods should still render
    expect(screen.getAllByText('Kustomization').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Deployment').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('Pods').length).toBeGreaterThanOrEqual(1)
  })

  it('should show pod summary with ready count and phase breakdown', () => {
    const mixedPods = [
      { name: 'pod-1', status: 'Running' },
      { name: 'pod-2', status: 'Pending' },
      { name: 'pod-3', status: 'Running' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={mixedPods}
      />
    )

    expect(screen.getAllByText('2/3 ready').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('2 running, 1 pending').length).toBeGreaterThanOrEqual(1)
  })

  it('should use lastAppliedRevision as fallback', () => {
    const reconcilerFallback = {
      ...baseReconciler,
      status: {
        ...baseReconciler.status,
        lastAttemptedRevision: undefined,
        lastAppliedRevision: 'main@sha1:fallback'
      }
    }

    render(
      <WorkloadPipelinePanel
        reconciler={reconcilerFallback}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    expect(screen.getAllByText('main@sha1:fallback').length).toBeGreaterThanOrEqual(1)
  })

  it('should format workload status for display', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    // 'Current' should be formatted as 'Ready' by formatWorkloadStatus
    expect(screen.getAllByText('Ready').length).toBeGreaterThanOrEqual(1)
  })

  it('should render connectors between nodes', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={basePods}
      />
    )

    const connectors = screen.getAllByTestId('pipeline-connector')
    // 3 connectors per layout x 2 layouts = 6
    expect(connectors).toHaveLength(6)
  })

  it('should show failed pod status border', () => {
    const failedPods = [
      { name: 'pod-1', status: 'Running' },
      { name: 'pod-2', status: 'Failed' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={failedPods}
      />
    )

    // Pods node should show "1/2 ready" with phase breakdown
    expect(screen.getAllByText('1/2 ready').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('1 running, 1 failed').length).toBeGreaterThanOrEqual(1)
  })

  it('should show completed count for CronJob pods', () => {
    const cronJobPods = [
      { name: 'job-abc-123', status: 'Succeeded' },
      { name: 'job-abc-456', status: 'Running' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="CronJob"
        name="backup"
        workloadStatus="Idle"
        pods={cronJobPods}
      />
    )

    expect(screen.getAllByText('1/2 completed').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('1 succeeded, 1 running').length).toBeGreaterThanOrEqual(1)
  })

  it('should show all completed for CronJob with succeeded pods', () => {
    const cronJobPods = [
      { name: 'job-abc-123', status: 'Succeeded' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="CronJob"
        name="backup"
        workloadStatus="Idle"
        pods={cronJobPods}
      />
    )

    expect(screen.getAllByText('1/1 completed').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('1 succeeded').length).toBeGreaterThanOrEqual(1)
  })

  it('should count failed CronJob pods as completed', () => {
    const failedCronPods = [
      { name: 'job-abc-123', status: 'Failed' },
      { name: 'job-abc-456', status: 'Failed' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="CronJob"
        name="backup"
        workloadStatus="Idle"
        pods={failedCronPods}
      />
    )

    expect(screen.getAllByText('2/2 completed').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('2 failed').length).toBeGreaterThanOrEqual(1)
  })

  it('should use Ready condition from podStatus for readiness', () => {
    // During a rollout: old pod is Running but Ready=False (terminating),
    // new pod is Pending with Ready=False
    const rolloutPods = [
      {
        name: 'nginx-old-abc',
        status: 'Running',
        podStatus: {
          conditions: [
            { type: 'Ready', status: 'False' }
          ]
        }
      },
      {
        name: 'nginx-new-def',
        status: 'Pending',
        podStatus: {
          conditions: [
            { type: 'Ready', status: 'False' }
          ]
        }
      }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="InProgress"
        pods={rolloutPods}
      />
    )

    // Neither pod is Ready, so 0/2 ready; phase summary shows actual phases
    expect(screen.getAllByText('0/2 ready').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('1 running, 1 pending').length).toBeGreaterThanOrEqual(1)
  })

  it('should count only pods with Ready=True condition as ready', () => {
    const mixedPods = [
      {
        name: 'nginx-ready',
        status: 'Running',
        podStatus: {
          conditions: [
            { type: 'Ready', status: 'True' }
          ]
        }
      },
      {
        name: 'nginx-not-ready',
        status: 'Running',
        podStatus: {
          conditions: [
            { type: 'Ready', status: 'False' }
          ]
        }
      }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="InProgress"
        pods={mixedPods}
      />
    )

    expect(screen.getAllByText('1/2 ready').length).toBeGreaterThanOrEqual(1)
  })

  it('should fall back to pod phase when podStatus is not available', () => {
    // No podStatus — falls back to phase-based check
    const simplePods = [
      { name: 'nginx-abc', status: 'Running' },
      { name: 'nginx-def', status: 'Pending' }
    ]

    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="InProgress"
        pods={simplePods}
      />
    )

    expect(screen.getAllByText('1/2 ready').length).toBeGreaterThanOrEqual(1)
  })

  it('should show scaled to zero for Deployment with no pods', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="Deployment"
        name="nginx"
        workloadStatus="Current"
        pods={[]}
      />
    )

    expect(screen.getAllByText('Scaled to zero').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('0 pods').length).toBeGreaterThanOrEqual(1)
  })

  it('should show no active pods for CronJob with no pods', () => {
    render(
      <WorkloadPipelinePanel
        reconciler={baseReconciler}
        kind="CronJob"
        name="backup"
        workloadStatus="Idle"
        pods={[]}
      />
    )

    expect(screen.getAllByText('No active pods').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText('0 pods').length).toBeGreaterThanOrEqual(1)
  })
})
