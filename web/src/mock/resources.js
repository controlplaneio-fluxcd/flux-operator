// Helper to generate timestamps within the last 2 hours (events expire after 2h)
const now = new Date()
const getTimestamp = (minutesAgo) => {
  const time = new Date(now.getTime() - minutesAgo * 60000)
  return time.toISOString()
}

export const mockResources = {
  resources: [
    // FluxInstance
    {
      name: "flux",
      kind: "FluxInstance",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 2s",
      lastReconciled: getTimestamp(5),
      inventory: [
        {
          name: "alerts.notification.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "artifactgenerators.source.extensions.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "buckets.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "externalartifacts.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "gitrepositories.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "helmcharts.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "helmreleases.helm.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "helmrepositories.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "kustomizations.kustomize.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "ocirepositories.source.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "providers.notification.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "receivers.notification.toolkit.fluxcd.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "flux-system",
          kind: "Namespace",
          apiVersion: "v1"
        },
        {
          name: "crd-controller-flux-system",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "flux-edit-flux-system",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "flux-view-flux-system",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "cluster-reconciler-flux-system",
          kind: "ClusterRoleBinding",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "crd-controller-flux-system",
          kind: "ClusterRoleBinding",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "critical-pods-flux-system",
          namespace: "flux-system",
          kind: "ResourceQuota",
          apiVersion: "v1"
        },
        {
          name: "helm-controller",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "kustomize-controller",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "notification-controller",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "source-controller",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "source-watcher",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "notification-controller",
          namespace: "flux-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "source-controller",
          namespace: "flux-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "source-watcher",
          namespace: "flux-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "webhook-receiver",
          namespace: "flux-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "helm-controller",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "kustomize-controller",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "notification-controller",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "source-controller",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "source-watcher",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "flux-system",
          namespace: "flux-system",
          kind: "Kustomization",
          apiVersion: "kustomize.toolkit.fluxcd.io/v1"
        },
        {
          name: "allow-egress",
          namespace: "flux-system",
          kind: "NetworkPolicy",
          apiVersion: "networking.k8s.io/v1"
        },
        {
          name: "allow-scraping",
          namespace: "flux-system",
          kind: "NetworkPolicy",
          apiVersion: "networking.k8s.io/v1"
        },
        {
          name: "allow-webhooks",
          namespace: "flux-system",
          kind: "NetworkPolicy",
          apiVersion: "networking.k8s.io/v1"
        },
        {
          name: "flux-system",
          namespace: "flux-system",
          kind: "GitRepository",
          apiVersion: "source.toolkit.fluxcd.io/v1"
        }
      ]
    },
    // ResourceSets
    {
      name: "flux-controllers",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "Reconciliation finished in 45ms",
      lastReconciled: getTimestamp(8),
      inventory: [
        {
          name: "flux-operator",
          namespace: "flux-system",
          kind: "Kustomization",
          apiVersion: "kustomize.toolkit.fluxcd.io/v1"
        },
        {
          name: "flux-operator",
          namespace: "flux-system",
          kind: "OCIRepository",
          apiVersion: "source.toolkit.fluxcd.io/v1"
        }
      ]
    },
    {
      name: "monitoring",
      kind: "ResourceSet",
      namespace: "flux-system",
      status: "Ready",
      message: "All resources reconciled successfully",
      lastReconciled: getTimestamp(75)
    },
    // ResourceSetInputProviders
    {
      name: "flux-config",
      kind: "ResourceSetInputProvider",
      namespace: "flux-system",
      status: "Ready",
      message: "Input synchronized from GitRepository/flux-system",
      lastReconciled: getTimestamp(10)
    },
    {
      name: "apps-config",
      kind: "ResourceSetInputProvider",
      namespace: "flux-system",
      status: "Progressing",
      message: "Input source temporarily unavailable, using cached version",
      lastReconciled: getTimestamp(55)
    },
    // Kustomizations
    {
      name: "flux-system",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Ready",
      message: "Applied revision: latest@sha256:a89edbf6e9600a8555ce0d7a12dd8ab7e6595f17770fa5af497f71ab96521961",
      lastReconciled: getTimestamp(15),
      inventory: [
        {
          name: "fluxinstances.fluxcd.controlplane.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "fluxreports.fluxcd.controlplane.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "resourcesetinputproviders.fluxcd.controlplane.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "resourcesets.fluxcd.controlplane.io",
          kind: "CustomResourceDefinition",
          apiVersion: "apiextensions.k8s.io/v1"
        },
        {
          name: "flux-operator-edit",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "flux-operator-view",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "flux-operator-cluster-admin",
          kind: "ClusterRoleBinding",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "flux-operator",
          namespace: "flux-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "flux-operator",
          namespace: "flux-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "flux-operator",
          namespace: "flux-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        }
      ]
    },
    {
      name: "apps",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Failed",
      message: "Reconciliation failed: failed to apply manifests - manifest validation failed for Deployment/podinfo",
      lastReconciled: getTimestamp(30)
    },
    {
      name: "infrastructure",
      kind: "Kustomization",
      namespace: "flux-system",
      status: "Ready",
      message: "Health assessment passed: all resources ready",
      lastReconciled: getTimestamp(65)
    },
    // HelmReleases
    {
      name: "podinfo",
      kind: "HelmRelease",
      namespace: "default",
      status: "Failed",
      message: "Helm install failed: failed to download chart from https://stefanprodan.github.io/podinfo",
      lastReconciled: getTimestamp(95)
    },
    {
      name: "ingress-nginx",
      kind: "HelmRelease",
      namespace: "kube-system",
      status: "Ready",
      message: "Helm upgrade succeeded, chart version: 6.5.4",
      lastReconciled: getTimestamp(12),
      inventory: [
        {
          name: "ingress-nginx",
          namespace: "kube-system",
          kind: "ServiceAccount",
          apiVersion: "v1"
        },
        {
          name: "ingress-nginx",
          kind: "ClusterRole",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "ingress-nginx",
          kind: "ClusterRoleBinding",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "ingress-nginx",
          namespace: "kube-system",
          kind: "Role",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "ingress-nginx",
          namespace: "kube-system",
          kind: "RoleBinding",
          apiVersion: "rbac.authorization.k8s.io/v1"
        },
        {
          name: "ingress-nginx-controller",
          namespace: "kube-system",
          kind: "ConfigMap",
          apiVersion: "v1"
        },
        {
          name: "ingress-nginx-controller",
          namespace: "kube-system",
          kind: "Service",
          apiVersion: "v1"
        },
        {
          name: "ingress-nginx-controller",
          namespace: "kube-system",
          kind: "Deployment",
          apiVersion: "apps/v1"
        },
        {
          name: "ingress-nginx-admission",
          namespace: "kube-system",
          kind: "Job",
          apiVersion: "batch/v1"
        },
        {
          name: "ingress-nginx-admission",
          namespace: "kube-system",
          kind: "ValidatingWebhookConfiguration",
          apiVersion: "admissionregistration.k8s.io/v1"
        }
      ]
    },
    {
      name: "cert-manager",
      kind: "HelmRelease",
      namespace: "cert-manager",
      status: "Ready",
      message: "Helm test completed successfully",
      lastReconciled: getTimestamp(40)
    },
    {
      name: "prometheus",
      kind: "HelmRelease",
      namespace: "monitoring",
      status: "Progressing",
      message: "Helm rollback initiated due to failed upgrade",
      lastReconciled: getTimestamp(80)
    },
    // GitRepositories
    {
      name: "flux-system",
      kind: "GitRepository",
      namespace: "flux-system",
      status: "Failed",
      message: "Failed to fetch artifact: connection timeout",
      lastReconciled: getTimestamp(35)
    },
    {
      name: "podinfo",
      kind: "GitRepository",
      namespace: "default",
      status: "Ready",
      message: "Stored artifact for revision: main@sha1:a7b3c2d",
      lastReconciled: getTimestamp(18)
    },
    {
      name: "apps",
      kind: "GitRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "Repository cloned successfully",
      lastReconciled: getTimestamp(60)
    },
    // OCIRepositories
    {
      name: "prometheus-config",
      kind: "OCIRepository",
      namespace: "monitoring",
      status: "Ready",
      message: "artifact up-to-date with remote revision: '0.33.0@sha256:5a303365'",
      lastReconciled: getTimestamp(20)
    },
    {
      name: "manifests",
      kind: "OCIRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "Pulled OCI artifact from ghcr.io/stefanprodan/manifests",
      lastReconciled: getTimestamp(70)
    },
    // HelmRepositories
    {
      name: "bitnami",
      kind: "HelmRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "Fetched index: 247 charts",
      lastReconciled: getTimestamp(22)
    },
    {
      name: "stable",
      kind: "HelmRepository",
      namespace: "flux-system",
      status: "Failed",
      message: "Repository index download failed: TLS handshake timeout",
      lastReconciled: getTimestamp(85)
    },
    // HelmCharts
    {
      name: "nginx",
      kind: "HelmChart",
      namespace: "flux-system",
      status: "Ready",
      message: "Pulled chart version 15.2.3",
      lastReconciled: getTimestamp(25)
    },
    {
      name: "prometheus",
      kind: "HelmChart",
      namespace: "monitoring",
      status: "Ready",
      message: "Chart package verified with digest sha256:xyz789",
      lastReconciled: getTimestamp(50)
    },
    // Buckets
    {
      name: "terraform-state",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Failed",
      message: "Failed to download bucket: access denied",
      lastReconciled: getTimestamp(42)
    },
    {
      name: "configs",
      kind: "Bucket",
      namespace: "flux-system",
      status: "Ready",
      message: "Bucket contents synchronized successfully",
      lastReconciled: getTimestamp(100)
    },
    // ImageRepositories
    {
      name: "podinfo",
      kind: "ImageRepository",
      namespace: "flux-system",
      status: "Ready",
      message: "Found 12 tags for image ghcr.io/stefanprodan/podinfo",
      lastReconciled: getTimestamp(28)
    },
    {
      name: "app",
      kind: "ImageRepository",
      namespace: "default",
      status: "Ready",
      message: "Scanning image repository for new tags",
      lastReconciled: getTimestamp(110)
    },
    // ImagePolicies
    {
      name: "podinfo",
      kind: "ImagePolicy",
      namespace: "flux-system",
      status: "Ready",
      message: "Latest image tag for policy semver:^6.x is: 6.5.4",
      lastReconciled: getTimestamp(32)
    },
    {
      name: "backend",
      kind: "ImagePolicy",
      namespace: "default",
      status: "Ready",
      message: "Policy evaluation successful, selected tag: v1.2.3",
      lastReconciled: getTimestamp(88)
    },
    // ImageUpdateAutomations
    {
      name: "flux-system",
      kind: "ImageUpdateAutomation",
      namespace: "flux-system",
      status: "Ready",
      message: "Committed and pushed change to branch main",
      lastReconciled: getTimestamp(48)
    },
    {
      name: "apps",
      kind: "ImageUpdateAutomation",
      namespace: "default",
      status: "Failed",
      message: "Failed to push commits: authentication failed",
      lastReconciled: getTimestamp(115)
    },
    // Alerts
    {
      name: "on-call-alerts",
      kind: "Alert",
      namespace: "flux-system",
      status: "Ready",
      message: "Notification sent to slack channel #flux-alerts",
      lastReconciled: getTimestamp(38)
    },
    // Providers
    {
      name: "slack",
      kind: "Provider",
      namespace: "flux-system",
      status: "Ready",
      message: "Provider initialized: Slack webhook configured",
      lastReconciled: getTimestamp(52)
    },
    // Receivers
    {
      name: "github-webhook",
      kind: "Receiver",
      namespace: "flux-system",
      status: "Ready",
      message: "Webhook received, triggered reconciliation for GitRepository/flux-system",
      lastReconciled: getTimestamp(78)
    },
    // ArtifactGenerators
    {
      name: "config-bundle",
      kind: "ArtifactGenerator",
      namespace: "flux-system",
      status: "Ready",
      message: "Generated artifact from ConfigMap/app-config",
      lastReconciled: getTimestamp(58)
    },
    // ExternalArtifacts
    {
      name: "vendor-manifests",
      kind: "ExternalArtifact",
      namespace: "flux-system",
      status: "Ready",
      message: "Artifact synchronized from external source",
      lastReconciled: getTimestamp(68)
    }
  ]
}
