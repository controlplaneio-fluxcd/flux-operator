// Mock FluxReport data for POC - matches config/testdata/flux-report.yaml
export const mockReport = {
  apiVersion: 'fluxcd.controlplane.io/v1',
  kind: 'FluxReport',
  metadata: {
    name: 'flux',
    namespace: 'flux-system'
  },
  spec: {
    cluster: {
      nodes: 2,
      platform: 'linux/arm64',
      serverVersion: 'v1.33.4'
    },
    components: [
      {
        image: 'ghcr.io/fluxcd/helm-controller:v1.4.3@sha256:d741dffd2a552b31cf215a1fcf1367ec7bc4dd3609b90e87595ae362d05d022c',
        name: 'helm-controller',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      },
      {
        image: 'ghcr.io/fluxcd/image-automation-controller:v1.0.3@sha256:2577ace8d1660b77df5297db239e9cf30520b336f9a74c3b4174d2773211319d',
        name: 'image-automation-controller',
        ready: false,
        status: 'Deployment ProgressDeadlineExceeded. ReplicaSet image-automation-controller-7d9b4c8f76 has failed to progress'
      },
      {
        image: 'ghcr.io/fluxcd/image-reflector-controller:v1.0.3@sha256:a5c718caddfae3022c109a6ef0eb6772a3cc6211aab39feca7c668dfeb151a2e',
        name: 'image-reflector-controller',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      },
      {
        image: 'ghcr.io/fluxcd/kustomize-controller:v1.7.2@sha256:477b4290a2fa2489bf87668bd7dcb77f0ae19bf944fef955600acbcde465ad98',
        name: 'kustomize-controller',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      },
      {
        image: 'ghcr.io/fluxcd/notification-controller:v1.7.4@sha256:350600b64cecb6cc10366c2bc41ec032fd604c81862298d02c303556a2fa6461',
        name: 'notification-controller',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      },
      {
        image: 'ghcr.io/fluxcd/source-controller:v1.7.3@sha256:5be9b7257270fa1a98c3c42af2f254a35bd64375e719090fe2ffc24915d8be06',
        name: 'source-controller',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      },
      {
        image: 'ghcr.io/fluxcd/source-watcher:v2.0.2@sha256:188a1adb89a16f7fcdd4ed79855301ec71950dcc833b6e0b3d0a053743ecac85',
        name: 'source-watcher',
        ready: true,
        status: 'Current Deployment is available. Replicas: 1'
      }
    ],
    distribution: {
      entitlement: 'Issued by controlplane',
      managedBy: 'flux-operator',
      status: 'Installed',
      version: 'v2.7.3'
    },
    operator: {
      apiVersion: 'fluxcd.controlplane.io/v1',
      platform: 'linux/arm64',
      version: 'v0.33.0'
    },
    reconcilers: [
      {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSet',
        stats: { failing: 0, running: 7, suspended: 0 }
      },
      {
        apiVersion: 'fluxcd.controlplane.io/v1',
        kind: 'ResourceSetInputProvider',
        stats: { failing: 0, running: 4, suspended: 1 }
      },
      {
        apiVersion: 'helm.toolkit.fluxcd.io/v2',
        kind: 'HelmRelease',
        stats: { failing: 0, running: 1600, suspended: 0 }
      },
      {
        apiVersion: 'image.toolkit.fluxcd.io/v1',
        kind: 'ImagePolicy',
        stats: { failing: 0, running: 80, suspended: 0 }
      },
      {
        apiVersion: 'image.toolkit.fluxcd.io/v1',
        kind: 'ImageRepository',
        stats: { failing: 0, running: 80, suspended: 0 }
      },
      {
        apiVersion: 'image.toolkit.fluxcd.io/v1',
        kind: 'ImageUpdateAutomation',
        stats: { failing: 0, running: 1, suspended: 0 }
      },
      {
        apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
        kind: 'Kustomization',
        stats: { failing: 1, running: 2200, suspended: 0 }
      },
      {
        apiVersion: 'notification.toolkit.fluxcd.io/v1',
        kind: 'Receiver',
        stats: { failing: 0, running: 1, suspended: 0 }
      },
      {
        apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
        kind: 'Alert',
        stats: { failing: 0, running: 20, suspended: 5 }
      },
      {
        apiVersion: 'notification.toolkit.fluxcd.io/v1beta3',
        kind: 'Provider',
        stats: { failing: 0, running: 25, suspended: 0 }
      },
      {
        apiVersion: 'source.extensions.fluxcd.io/v1beta1',
        kind: 'ArtifactGenerator',
        stats: { failing: 0, running: 2, suspended: 0 }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'Bucket',
        stats: { failing: 1, running: 3, suspended: 2 }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'ExternalArtifact',
        stats: { failing: 0, running: 5, suspended: 0 }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'GitRepository',
        stats: { failing: 0, running: 1, suspended: 0, totalSize: '11.9 KiB' }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'HelmChart',
        stats: { failing: 0, running: 416, suspended: 0, totalSize: '50.6 KiB' }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'HelmRepository',
        stats: { failing: 0, running: 10, suspended: 0, totalSize: '75.6 KiB' }
      },
      {
        apiVersion: 'source.toolkit.fluxcd.io/v1',
        kind: 'OCIRepository',
        stats: { failing: 0, running: 623, suspended: 0, totalSize: '1.0 MiB' }
      }
    ],
    sync: {
      id: 'kustomization/flux-system',
      path: './clusters/homelab',
      ready: false,
      source: 'https://github.com/stefanprodan/homelab.git',
      // status: 'Applied revision: refs/heads/main@sha1:96b331c8f63315590a68c18290182a66c49d2d1e'
      status: 'error decrypting sources:\nSTS: AssumeRoleWithWebIdentity, https response error\nPost "https://sts.arn.amazonaws.com/": dial tcp: lookupts.arn.amazonaws.com on 10.100.0.10:53: no such host'
    },
    metrics: [
      {
        pod: 'helm-controller-77c6f47d9c-qn2hq',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.7485,
        memory: 912680550,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'image-automation-controller-585ddb54cf-8cjdh',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.0021,
        memory: 650000000,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'image-reflector-controller-66f965d7c4-htrkq',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.0458,
        memory: 750000000,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'kustomize-controller-5cb6bb5948-tgmgz',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.8234,
        memory: 944892805,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'notification-controller-79b4d898bd-n2n8n',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.0018,
        memory: 700000000,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'source-controller-6c97954754-hglpv',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.0083,
        memory: 800000000,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      },
      {
        pod: 'source-watcher-758b694c5d-7jg7m',
        namespace: 'flux-system',
        container: 'manager',
        cpu: 0.0035,
        memory: 600000000,
        cpuLimit: 1.0,
        memoryLimit: 1073741824
      }
    ]
  }
}
