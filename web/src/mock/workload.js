// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for workload endpoint (GET /api/v1/workload)
// Generated from real cluster API responses

// Helper to generate timestamps
const now = new Date()
const getTimestamp = (daysAgo, hoursAgo = 0, minutesAgo = 0, secondsAgo = 0) => {
  const time = new Date(now.getTime() - ((daysAgo * 24 * 60 + hoursAgo * 60 + minutesAgo) * 60 + secondsAgo) * 1000)
  return time.toISOString()
}

// Mock workload data
// Pod statuses use Kubernetes pod phases: Pending, Running, Succeeded, Failed, Unknown
const mockWorkloads = {
  // Flux controllers in flux-system namespace
  'Deployment/flux-system/source-controller': {
    kind: 'Deployment',
    name: 'source-controller',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    createdAt: getTimestamp(30, 0, 0), // 30 days ago
    restartedAt: getTimestamp(7, 2, 15), // Last restarted 7 days ago
    containerImages: [
      'ghcr.io/fluxcd/source-controller:v1.7.4@sha256:16f21ac1795528df80ddef51ccbb14a57b78ea26e66dc8551636ef9a3cec71b3'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'source-controller-5f76f5c549-wz2gk',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 2, 15) // 7 days, 2 hours, 15 minutes ago
      }
    ]
  },
  'Deployment/flux-system/kustomize-controller': {
    kind: 'Deployment',
    name: 'kustomize-controller',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    createdAt: getTimestamp(30, 0, 0), // 30 days ago
    containerImages: [
      'ghcr.io/fluxcd/kustomize-controller:v1.7.3@sha256:e8ca82d66dafdd8ef77e0917f4adec53478075130ac61264dc0f91eb0f8cb6ce'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'kustomize-controller-5fc57fb9cc-bhl8q',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 2, 20)
      }
    ]
  },
  'Deployment/flux-system/helm-controller': {
    kind: 'Deployment',
    name: 'helm-controller',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    createdAt: getTimestamp(30, 0, 0),
    containerImages: [
      'ghcr.io/fluxcd/helm-controller:v1.4.4@sha256:5eae73909e1471c0cd01bb23d87c9d4219a4f645134a23629c8708c72635398d'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'helm-controller-bf4685d7f-nxqsj',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 2, 25)
      }
    ]
  },
  'Deployment/flux-system/notification-controller': {
    kind: 'Deployment',
    name: 'notification-controller',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'ghcr.io/fluxcd/notification-controller:v1.7.5@sha256:ba723a55f7c7c7feedd50bb5db0ff2dd9a3b0ae85b50f61a0457184025b38c54'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'notification-controller-58cfb55954-fcf6l',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 2, 30)
      }
    ]
  },
  'Deployment/flux-system/image-automation-controller': {
    kind: 'Deployment',
    name: 'image-automation-controller',
    namespace: 'flux-system',
    status: 'InProgress',
    statusMessage: 'Waiting for rollout to finish: 0 of 1 updated replicas are available',
    createdAt: getTimestamp(30, 0, 0),
    restartedAt: getTimestamp(0, 0, 0, 10), // Restarted 10 seconds ago (recent, still in progress)
    containerImages: [
      'ghcr.io/fluxcd/image-automation-controller:v1.0.4@sha256:f9383dccb80ec65e274648941af623ce74084d25026e14389111c14b630efece'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'image-automation-controller-5c5fc5487b-w4458',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(5, 8, 45),
        podStatus: {
          phase: 'Running',
          containerStatuses: [
            {
              name: 'manager',
              ready: true,
              restartCount: 0,
              imageID: 'ghcr.io/fluxcd/image-automation-controller:v1.0.4@sha256:f9383dccb80ec65e274648941af623ce74084d25026e14389111c14b630efece',
              state: { running: { startedAt: getTimestamp(5, 8, 45) } }
            }
          ],
          conditions: [
            { type: 'Ready', status: 'True' },
            { type: 'ContainersReady', status: 'True' },
            { type: 'PodScheduled', status: 'True' }
          ]
        }
      },
      {
        name: 'image-automation-controller-dfcfc789b-9dtqk',
        status: 'Pending',
        statusMessage: 'Waiting: ImagePullBackOff',
        createdAt: getTimestamp(0, 0, 5), // Recent pod with issue
        podStatus: {
          phase: 'Pending',
          containerStatuses: [
            {
              name: 'manager',
              ready: false,
              restartCount: 3,
              imageID: '',
              state: {
                waiting: {
                  reason: 'ImagePullBackOff',
                  message: 'Back-off pulling image "ghcr.io/fluxcd/image-automation-controller:v1.0.5"'
                }
              }
            }
          ],
          conditions: [
            { type: 'PodScheduled', status: 'True' },
            { type: 'Initialized', status: 'True' },
            { type: 'ContainersReady', status: 'False', reason: 'ContainersNotReady', message: 'containers with unready status: [manager]' },
            { type: 'Ready', status: 'False', reason: 'ContainersNotReady', message: 'containers with unready status: [manager]' }
          ]
        }
      }
    ]
  },
  'Deployment/flux-system/image-reflector-controller': {
    kind: 'Deployment',
    name: 'image-reflector-controller',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'ghcr.io/fluxcd/image-reflector-controller:v1.0.4@sha256:0bdc30aea2b7cdfea02d0f6d53c06b9df0ea1c6516b85ed523792e222329c039'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'image-reflector-controller-547c8dbffc-2gjhj',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 3, 10)
      }
    ]
  },
  'Deployment/flux-system/source-watcher': {
    kind: 'Deployment',
    name: 'source-watcher',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'ghcr.io/fluxcd/source-watcher:v2.0.3@sha256:9cd46c3c958dcfcd8a3c857fa09989f9df5d8396eae165f219cbb472343371a9'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'source-watcher-85bcf4bd57-vfbs6',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 3, 20)
      }
    ]
  },
  'Deployment/flux-system/flux-operator': {
    kind: 'Deployment',
    name: 'flux-operator',
    namespace: 'flux-system',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'ghcr.io/controlplaneio-fluxcd/flux-operator:v0.34.0'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'flux-operator-67cdfc557d-h656w',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(7, 3, 30)
      }
    ]
  },

  // cert-manager namespace
  'Deployment/cert-manager/cert-manager': {
    kind: 'Deployment',
    name: 'cert-manager',
    namespace: 'cert-manager',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'quay.io/jetstack/cert-manager-controller:v1.19.1'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'cert-manager-6b7bcdbb84-cclfj',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(10, 4, 15)
      }
    ]
  },
  'Deployment/cert-manager/cert-manager-cainjector': {
    kind: 'Deployment',
    name: 'cert-manager-cainjector',
    namespace: 'cert-manager',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'quay.io/jetstack/cert-manager-cainjector:v1.19.1'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'cert-manager-cainjector-d74c65ddb-6v869',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(10, 4, 20)
      }
    ]
  },
  'Deployment/cert-manager/cert-manager-webhook': {
    kind: 'Deployment',
    name: 'cert-manager-webhook',
    namespace: 'cert-manager',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'quay.io/jetstack/cert-manager-webhook:v1.19.1'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'cert-manager-webhook-6bf5dfc659-w95d9',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(0, 0, 0)
      }
    ]
  },

  // Other namespaces
  'Deployment/monitoring/metrics-server': {
    kind: 'Deployment',
    name: 'metrics-server',
    namespace: 'monitoring',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'registry.k8s.io/metrics-server/metrics-server:v0.8.0'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'metrics-server-57b56685f4-59gn2',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(14, 6, 30)
      }
    ]
  },
  'Deployment/tailscale/operator': {
    kind: 'Deployment',
    name: 'operator',
    namespace: 'tailscale',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    containerImages: [
      'tailscale/k8s-operator:v1.90.8'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'operator-84ddf77c66-gjsxz',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(12, 8, 45)
      }
    ]
  },
  'StatefulSet/registry/zot-registry': {
    kind: 'StatefulSet',
    name: 'zot-registry',
    namespace: 'registry',
    status: 'Current',
    statusMessage: 'Replicas: 1',
    createdAt: getTimestamp(60, 0, 0), // 60 days ago
    containerImages: [
      'ghcr.io/project-zot/zot:v2.1.11'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'zot-registry-0',
        status: 'Running',
        statusMessage: 'Started at 2026-01-26 09:45:00 UTC',
        createdAt: getTimestamp(15, 10, 0)
      }
    ]
  },

  // CronJob examples
  'CronJob/flux-system/garbage-collection': {
    kind: 'CronJob',
    name: 'garbage-collection',
    namespace: 'flux-system',
    status: 'Idle',
    statusMessage: '0 */6 * * *',
    createdAt: getTimestamp(45, 0, 0), // 45 days ago
    containerImages: [
      'ghcr.io/fluxcd/flux-cli:v2.6.1'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'garbage-collection-28945678-xk9j2',
        status: 'Running',
        statusMessage: 'Started at 2026-02-06 10:30:00 UTC',
        createdAt: getTimestamp(0, 0, 0, 10), // 10 seconds ago (recent, in progress)
        createdBy: 'admin@example.com'
      }
    ]
  },
  'CronJob/monitoring/prometheus-backup': {
    kind: 'CronJob',
    name: 'prometheus-backup',
    namespace: 'monitoring',
    status: 'Idle',
    statusMessage: '0 0 * * *',
    containerImages: [
      'prom/prometheus:v3.3.0'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'prometheus-backup-28945600-abc12',
        status: 'Succeeded',
        statusMessage: 'Completed at 2026-02-02 06:00:00 UTC',
        createdAt: getTimestamp(0, 0, 0) // now
      }
    ]
  },
  'CronJob/cert-manager/cert-renewal-check': {
    kind: 'CronJob',
    name: 'cert-renewal-check',
    namespace: 'cert-manager',
    status: 'Failed',
    statusMessage: 'Job failed: BackoffLimitExceeded',
    containerImages: [
      'quay.io/jetstack/cert-manager-ctl:v1.19.1'
    ],
    userActions: ['deletePods'],
    pods: [
      {
        name: 'cert-renewal-check-28945500-def34',
        status: 'Failed',
        statusMessage: 'Reason: Error',
        createdAt: getTimestamp(0, 12, 0), // 12 hours ago
        podStatus: {
          phase: 'Failed',
          containerStatuses: [
            {
              name: 'cert-check',
              ready: false,
              restartCount: 3,
              imageID: 'quay.io/jetstack/cert-manager-ctl:v1.19.1@sha256:abc123def456',
              state: {
                terminated: {
                  reason: 'Error',
                  exitCode: 1,
                  startedAt: getTimestamp(0, 12, 0),
                  finishedAt: getTimestamp(0, 11, 55)
                }
              }
            }
          ],
          conditions: [
            { type: 'PodScheduled', status: 'True' },
            { type: 'Initialized', status: 'True' },
            { type: 'ContainersReady', status: 'False', reason: 'PodFailed' },
            { type: 'Ready', status: 'False', reason: 'PodFailed' }
          ]
        }
      }
    ]
  }
}

// Mock reconciler data for workload detail view.
// Maps "ReconcilerKind/Namespace/Name" to the enriched Flux resource.
const mockReconcilers = {
  'Kustomization/flux-system/flux-controllers': {
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    kind: 'Kustomization',
    metadata: { name: 'flux-controllers', namespace: 'flux-system' },
    spec: { interval: '10m', prune: true },
    status: {
      reconcilerRef: { status: 'Ready', message: 'Applied revision: main@sha1:9b9218f' },
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system',
        namespace: 'flux-system',
        status: 'Ready',
        url: 'https://github.com/example/fleet-infra'
      },
      lastAttemptedRevision: 'main@sha1:9b9218f'
    }
  },
  'HelmRelease/cert-manager/cert-manager': {
    apiVersion: 'helm.toolkit.fluxcd.io/v2',
    kind: 'HelmRelease',
    metadata: { name: 'cert-manager', namespace: 'cert-manager' },
    spec: { interval: '30m' },
    status: {
      reconcilerRef: { status: 'Ready', message: 'Helm install succeeded' },
      sourceRef: {
        kind: 'HelmRepository',
        name: 'jetstack',
        namespace: 'cert-manager',
        status: 'Ready',
        url: 'https://charts.jetstack.io'
      },
      lastAttemptedRevision: 'v1.19.1'
    }
  },
  'Kustomization/flux-system/monitoring': {
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    kind: 'Kustomization',
    metadata: { name: 'monitoring', namespace: 'flux-system' },
    spec: { interval: '15m', prune: true },
    status: {
      reconcilerRef: { status: 'Ready', message: 'Applied revision: main@sha1:03bba48' },
      sourceRef: {
        kind: 'GitRepository',
        name: 'flux-system',
        namespace: 'flux-system',
        status: 'Ready',
        url: 'https://github.com/example/fleet-infra'
      },
      lastAttemptedRevision: 'main@sha1:03bba48'
    }
  }
}

// Maps workload keys to their parent reconciler keys
const workloadReconcilerMap = {
  'Deployment/flux-system/source-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/kustomize-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/helm-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/notification-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/image-automation-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/image-reflector-controller': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/source-watcher': 'Kustomization/flux-system/flux-controllers',
  'Deployment/flux-system/flux-operator': 'Kustomization/flux-system/flux-controllers',
  'Deployment/cert-manager/cert-manager': 'HelmRelease/cert-manager/cert-manager',
  'Deployment/cert-manager/cert-manager-cainjector': 'HelmRelease/cert-manager/cert-manager',
  'Deployment/cert-manager/cert-manager-webhook': 'HelmRelease/cert-manager/cert-manager',
  'Deployment/monitoring/metrics-server': 'Kustomization/flux-system/monitoring',
  'Deployment/tailscale/operator': 'Kustomization/flux-system/monitoring',
  'StatefulSet/registry/zot-registry': 'Kustomization/flux-system/monitoring',
  'CronJob/flux-system/garbage-collection': 'Kustomization/flux-system/flux-controllers',
  'CronJob/monitoring/prometheus-backup': 'Kustomization/flux-system/monitoring',
  'CronJob/cert-manager/cert-renewal-check': 'HelmRelease/cert-manager/cert-manager'
}

/**
 * Build a mock API response for the workload detail endpoint (GET /api/v1/workload).
 * Wraps the flat workload data into the shape returned by the Go backend:
 * the full Kubernetes resource with workloadInfo (status, pods, reconciler) injected.
 * @param {object} workload - Flat workload data from mockWorkloads
 * @param {object|undefined} reconciler - Optional reconciler mock data
 * @returns {object} - Mock API response matching GetWorkloadDetails format
 */
function buildWorkloadDetailResponse(workload, reconciler) {
  const apiVersion = workload.kind === 'CronJob' ? 'batch/v1' : 'apps/v1'

  return {
    apiVersion,
    kind: workload.kind,
    metadata: {
      name: workload.name,
      namespace: workload.namespace,
      creationTimestamp: workload.createdAt
    },
    spec: {},
    status: {},
    workloadInfo: {
      status: workload.status,
      statusMessage: workload.statusMessage,
      createdAt: workload.createdAt,
      restartedAt: workload.restartedAt,
      containerImages: workload.containerImages,
      userActions: workload.userActions,
      pods: workload.pods,
      reconciler: reconciler || undefined
    }
  }
}

/**
 * Get mock workload by kind, name, and namespace
 * This function is called by fetchWithMock when in mock mode
 * @param {string} endpoint - The API endpoint with query parameters
 * @returns {object} - Mock workload data matching GetWorkloadDetails response format
 */
export function getMockWorkload(endpoint) {
  // Parse query parameters from endpoint
  const queryString = endpoint.includes('?') ? endpoint.split('?')[1] : ''
  const params = new URLSearchParams(queryString)
  const kind = params.get('kind')
  const name = params.get('name')
  const namespace = params.get('namespace')

  if (!kind || !name || !namespace) {
    console.warn('getMockWorkload: Missing required parameters (kind, name, namespace)')
    return {}
  }

  const key = `${kind}/${namespace}/${name}`
  const workload = mockWorkloads[key]

  if (!workload) {
    console.warn(`getMockWorkload: No mock data found for ${key}`)
    return {}
  }

  const reconcilerKey = workloadReconcilerMap[key]
  const reconciler = reconcilerKey ? mockReconcilers[reconcilerKey] : undefined

  return buildWorkloadDetailResponse(workload, reconciler)
}

/**
 * Get mock workloads for batch request (POST /api/v1/workloads)
 * @param {object} body - Request body with workloads array
 * @returns {object} - Mock response with workloads array
 */
export function getMockWorkloads(body) {
  const requestedWorkloads = body?.workloads || []
  if (requestedWorkloads.length === 0) {
    return { workloads: [] }
  }

  const results = []
  for (const item of requestedWorkloads) {
    const key = `${item.kind}/${item.namespace}/${item.name}`
    const workload = mockWorkloads[key]
    if (workload) {
      results.push(workload)
    } else {
      // Return NotFound status for missing workloads
      results.push({
        kind: item.kind,
        name: item.name,
        namespace: item.namespace,
        status: 'NotFound',
        statusMessage: 'Workload not found in cluster'
      })
    }
  }

  return { workloads: results }
}
