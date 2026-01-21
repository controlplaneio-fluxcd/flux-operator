// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

// Mock data for individual resource endpoint (GET /api/v1/resource)
// Generated from real cluster API responses

// Array containing all mock resources
export const mockResourcesArray =
[
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "FluxInstance",
    "metadata": {
      "name": "flux",
      "namespace": "flux-system"
    },
    "spec": {
      "cluster": {
        "domain": "cluster.local",
        "multitenant": false,
        "multitenantWorkloadIdentity": false,
        "networkPolicy": true,
        "size": "medium",
        "type": "kubernetes"
      },
      "components": [
        "source-controller",
        "source-watcher",
        "kustomize-controller",
        "helm-controller",
        "notification-controller",
        "image-reflector-controller",
        "image-automation-controller"
      ],
      "distribution": {
        "artifact": "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests:latest",
        "registry": "ghcr.io/fluxcd",
        "version": "2.x"
      },
      "migrateResources": true,
      "sync": {
        "interval": "1m",
        "kind": "GitRepository",
        "path": "./clusters/homelab",
        "pullSecret": "github-auth",
        "ref": "refs/heads/main",
        "url": "https://github.com/stefanprodan/homelab.git"
      },
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 2s",
        "lastReconciled": "2025-11-18T11:10:59Z",
        "managedBy": ""
      },
      "components": [
        {
          "digest": "sha256:5be9b7257270fa1a98c3c42af2f254a35bd64375e719090fe2ffc24915d8be06",
          "name": "source-controller",
          "repository": "ghcr.io/fluxcd/source-controller",
          "tag": "v1.7.3"
        },
        {
          "digest": "sha256:188a1adb89a16f7fcdd4ed79855301ec71950dcc833b6e0b3d0a053743ecac85",
          "name": "source-watcher",
          "repository": "ghcr.io/fluxcd/source-watcher",
          "tag": "v2.0.2"
        },
        {
          "digest": "sha256:477b4290a2fa2489bf87668bd7dcb77f0ae19bf944fef955600acbcde465ad98",
          "name": "kustomize-controller",
          "repository": "ghcr.io/fluxcd/kustomize-controller",
          "tag": "v1.7.2"
        },
        {
          "digest": "sha256:d741dffd2a552b31cf215a1fcf1367ec7bc4dd3609b90e87595ae362d05d022c",
          "name": "helm-controller",
          "repository": "ghcr.io/fluxcd/helm-controller",
          "tag": "v1.4.3"
        },
        {
          "digest": "sha256:350600b64cecb6cc10366c2bc41ec032fd604c81862298d02c303556a2fa6461",
          "name": "notification-controller",
          "repository": "ghcr.io/fluxcd/notification-controller",
          "tag": "v1.7.4"
        },
        {
          "digest": "sha256:a5c718caddfae3022c109a6ef0eb6772a3cc6211aab39feca7c668dfeb151a2e",
          "name": "image-reflector-controller",
          "repository": "ghcr.io/fluxcd/image-reflector-controller",
          "tag": "v1.0.3"
        },
        {
          "digest": "sha256:2577ace8d1660b77df5297db239e9cf30520b336f9a74c3b4174d2773211319d",
          "name": "image-automation-controller",
          "repository": "ghcr.io/fluxcd/image-automation-controller",
          "tag": "v1.0.3"
        }
      ],
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:10:59Z",
          "message": "Reconciliation finished in 2s",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:d8fc4a3a7b06f200e0565c35d694b9d36cbe43233d8418a78525adc3ca9a16dd",
          "firstReconciled": "2025-11-01T23:31:08Z",
          "lastReconciled": "2025-11-18T11:10:59Z",
          "lastReconciledDuration": "1.605875375s",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "flux": "v2.7.3"
          },
          "totalReconciliations": 397
        }
      ],
      "inventory": [
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "alerts.notification.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "artifactgenerators.source.extensions.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "buckets.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "externalartifacts.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "gitrepositories.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "helmcharts.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "helmreleases.helm.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "helmrepositories.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "imagepolicies.image.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "imagerepositories.image.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "imageupdateautomations.image.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "kustomizations.kustomize.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "ocirepositories.source.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "providers.notification.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "receivers.notification.toolkit.fluxcd.io",
          "namespace": ""
        },
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "name": "flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "crd-controller-flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "flux-edit-flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "flux-view-flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cluster-reconciler-flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "crd-controller-flux-system",
          "namespace": ""
        },
        {
          "apiVersion": "v1",
          "kind": "ResourceQuota",
          "name": "critical-pods-flux-system",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "helm-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "image-automation-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "image-reflector-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "kustomize-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "notification-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "source-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "source-watcher",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "notification-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "source-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "source-watcher",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "webhook-receiver",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "helm-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "image-automation-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "image-reflector-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "kustomize-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "notification-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "source-controller",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "source-watcher",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
          "kind": "Kustomization",
          "name": "flux-system",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "NetworkPolicy",
          "name": "allow-egress",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "NetworkPolicy",
          "name": "allow-scraping",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "NetworkPolicy",
          "name": "allow-webhooks",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "GitRepository",
          "name": "flux-system",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "v2.7.3@sha256:d8fc4a3a7b06f200e0565c35d694b9d36cbe43233d8418a78525adc3ca9a16dd",
      "lastArtifactRevision": "sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
      "lastAttemptedRevision": "v2.7.3@sha256:d8fc4a3a7b06f200e0565c35d694b9d36cbe43233d8418a78525adc3ca9a16dd",
      "lastHandledReconcileAt": "2025-11-07T12:20:38.600281345Z"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "GitRepository",
    "metadata": {
      "annotations": {
        "kustomize.toolkit.fluxcd.io/prune": "Disabled",
        "kustomize.toolkit.fluxcd.io/ssa": "Ignore"
      },
      "labels": {
        "app.kubernetes.io/instance": "flux-system",
        "app.kubernetes.io/managed-by": "flux-operator",
        "app.kubernetes.io/part-of": "flux",
        "app.kubernetes.io/version": "v2.7.3"
      },
      "name": "flux-system",
      "namespace": "flux-system"
    },
    "spec": {
      "interval": "1m0s",
      "ref": {
        "name": "refs/heads/main"
      },
      "secretRef": {
        "name": "github-auth"
      },
      "timeout": "60s",
      "url": "https://github.com/stefanprodan/homelab.git"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
        "lastReconciled": "2025-11-11T14:07:19Z",
        "managedBy": "FluxInstance/flux-system/flux"
      },
      "artifact": {
        "digest": "sha256:fe0450de125a2359c0e14106830910855756014b0f78b4cf3b21339505a5bf74",
        "lastUpdateTime": "2025-11-06T21:36:36Z",
        "path": "gitrepository/flux-system/flux-system/d676e33990dc2865d67c022d26dea93d5e3236ff.tar.gz",
        "revision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
        "size": 13059,
        "url": "http://source-controller.flux-system.svc.cluster.local./gitrepository/flux-system/flux-system/d676e33990dc2865d67c022d26dea93d5e3236ff.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-11T14:07:19Z",
          "message": "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-06T21:36:36Z",
          "message": "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "lastHandledReconcileAt": "2025-11-06T23:35:15.038685+02:00",
      "observedGeneration": 1
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "HelmChart",
    "metadata": {
      "name": "registry-zot-registry",
      "namespace": "registry"
    },
    "spec": {
      "chart": "zot",
      "interval": "24h0m0s",
      "reconcileStrategy": "ChartVersion",
      "sourceRef": {
        "kind": "HelmRepository",
        "name": "zot-registry"
      },
      "version": "*"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "pulled 'zot' chart with version '0.1.89'",
        "lastReconciled": "2025-11-01T23:31:41Z",
        "managedBy": ""
      },
      "artifact": {
        "digest": "sha256:8c3ab9d44828c879be47874f0c3dca2603a0f0584ef383c18ad50a4a27829d67",
        "lastUpdateTime": "2025-11-01T23:31:41Z",
        "path": "helmchart/registry/registry-zot-registry/zot-0.1.89.tgz",
        "revision": "0.1.89",
        "size": 9429,
        "url": "http://source-controller.flux-system.svc.cluster.local./helmchart/registry/registry-zot-registry/zot-0.1.89.tgz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:41Z",
          "message": "pulled 'zot' chart with version '0.1.89'",
          "observedGeneration": 1,
          "reason": "ChartPullSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:41Z",
          "message": "pulled 'zot' chart with version '0.1.89'",
          "observedGeneration": 1,
          "reason": "ChartPullSucceeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedChartName": "zot",
      "observedGeneration": 1,
      "observedSourceArtifactRevision": "sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125",
      "url": "http://source-controller.flux-system.svc.cluster.local./helmchart/registry/registry-zot-registry/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "HelmChart",
    "metadata": {
      "name": "tailscale-tailscale-operator",
      "namespace": "tailscale"
    },
    "spec": {
      "chart": "tailscale-operator",
      "interval": "24h0m0s",
      "reconcileStrategy": "ChartVersion",
      "sourceRef": {
        "kind": "HelmRepository",
        "name": "tailscale-operator"
      },
      "version": "*"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "pulled 'tailscale-operator' chart with version '1.90.6'",
        "lastReconciled": "2025-11-01T23:31:02Z",
        "managedBy": ""
      },
      "artifact": {
        "digest": "sha256:08247dd90325a32ae95c5b116917015458b569eadbd66e7116dcdc7502a82bd9",
        "lastUpdateTime": "2025-11-01T23:31:02Z",
        "path": "helmchart/tailscale/tailscale-tailscale-operator/tailscale-operator-1.90.6.tgz",
        "revision": "1.90.6",
        "size": 42432,
        "url": "http://source-controller.flux-system.svc.cluster.local./helmchart/tailscale/tailscale-tailscale-operator/tailscale-operator-1.90.6.tgz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:02Z",
          "message": "pulled 'tailscale-operator' chart with version '1.90.6'",
          "observedGeneration": 1,
          "reason": "ChartPullSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:02Z",
          "message": "pulled 'tailscale-operator' chart with version '1.90.6'",
          "observedGeneration": 1,
          "reason": "ChartPullSucceeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedChartName": "tailscale-operator",
      "observedGeneration": 1,
      "observedSourceArtifactRevision": "sha256:578d082975ad264ba4d09368febb298c3beb7f18e459bb9d323d3b7c2fc4d475",
      "url": "http://source-controller.flux-system.svc.cluster.local./helmchart/tailscale/tailscale-tailscale-operator/latest.tar.gz"
    }
  },
  {
    "apiVersion": "helm.toolkit.fluxcd.io/v2",
    "kind": "HelmRelease",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "cert-manager"
      },
      "name": "cert-manager",
      "namespace": "cert-manager"
    },
    "spec": {
      "chartRef": {
        "kind": "OCIRepository",
        "name": "cert-manager"
      },
      "install": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "2m"
        }
      },
      "interval": "24h",
      "releaseName": "cert-manager",
      "upgrade": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "3m"
        }
      },
      "values": {
        "crds": {
          "enabled": true,
          "keep": false
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Helm install succeeded for release cert-manager/cert-manager.v1 with chart cert-manager@1.19.1+9578566b26b2",
        "lastReconciled": "2025-11-01T23:31:36Z",
        "managedBy": "ResourceSet/flux-system/cert-manager"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:36Z",
          "message": "Helm install succeeded for release cert-manager/cert-manager.v1 with chart cert-manager@1.19.1+9578566b26b2",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:36Z",
          "message": "Helm install succeeded for release cert-manager/cert-manager.v1 with chart cert-manager@1.19.1+9578566b26b2",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Released"
        }
      ],
      "history": [
        {
          "appVersion": "v1.19.1",
          "chartName": "cert-manager",
          "chartVersion": "1.19.1+9578566b26b2",
          "configDigest": "sha256:c4de42eac3a72305e609d1d9de974488e24310a5241ed6eea8da43b39042a4d0",
          "digest": "sha256:b6eda3ef50097eb669b52c31d0b9824ef2303373784ebd4f231675efbc23b82b",
          "firstDeployed": "2025-11-01T23:31:03Z",
          "lastDeployed": "2025-11-01T23:31:03Z",
          "name": "cert-manager",
          "namespace": "cert-manager",
          "ociDigest": "sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9",
          "status": "deployed",
          "version": 1
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "cert-manager-cainjector",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "cert-manager",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "cert-manager-webhook",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "challenges.acme.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "orders.acme.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "certificaterequests.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "certificates.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "clusterissuers.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "issuers.cert-manager.io",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-cainjector",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-issuers",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-clusterissuers",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-certificates",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-orders",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-challenges",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-ingress-shim",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-cluster-view",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-view",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-edit",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-approve:cert-manager-io",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-controller-certificatesigningrequests",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "cert-manager-webhook:subjectaccessreviews",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-cainjector",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-issuers",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-clusterissuers",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-certificates",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-orders",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-challenges",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-ingress-shim",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-approve:cert-manager-io",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-controller-certificatesigningrequests",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "cert-manager-webhook:subjectaccessreviews",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "cert-manager-cainjector:leaderelection",
          "namespace": "kube-system"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "cert-manager:leaderelection",
          "namespace": "kube-system"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "cert-manager-tokenrequest",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "cert-manager-webhook:dynamic-serving",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "cert-manager-cainjector:leaderelection",
          "namespace": "kube-system"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "cert-manager:leaderelection",
          "namespace": "kube-system"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "cert-manager-tokenrequest",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "cert-manager-webhook:dynamic-serving",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "cert-manager-cainjector",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "cert-manager",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "cert-manager-webhook",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "cert-manager-cainjector",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "cert-manager",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "cert-manager-webhook",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "admissionregistration.k8s.io/v1",
          "kind": "MutatingWebhookConfiguration",
          "name": "cert-manager-webhook",
          "namespace": ""
        },
        {
          "apiVersion": "admissionregistration.k8s.io/v1",
          "kind": "ValidatingWebhookConfiguration",
          "name": "cert-manager-webhook",
          "namespace": ""
        }
      ],
      "lastAttemptedConfigDigest": "sha256:c4de42eac3a72305e609d1d9de974488e24310a5241ed6eea8da43b39042a4d0",
      "lastAttemptedGeneration": 1,
      "lastAttemptedReleaseAction": "install",
      "lastAttemptedReleaseActionDuration": "32.702330598s",
      "lastAttemptedRevision": "1.19.1+9578566b26b2",
      "lastAttemptedRevisionDigest": "sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "OCIRepository",
        "message": "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
        "name": "cert-manager",
        "namespace": "cert-manager",
        "originRevision": "",
        "originURL": "https://github.com/cert-manager/cert-manager",
        "status": "Ready",
        "url": "oci://quay.io/jetstack/charts/cert-manager"
      },
      "storageNamespace": "cert-manager"
    }
  },
  {
    "apiVersion": "helm.toolkit.fluxcd.io/v2",
    "kind": "HelmRelease",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "metrics-server"
      },
      "name": "metrics-server",
      "namespace": "monitoring"
    },
    "spec": {
      "chartRef": {
        "kind": "OCIRepository",
        "name": "metrics-server"
      },
      "install": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "2m"
        }
      },
      "interval": "24h",
      "releaseName": "metrics-server",
      "upgrade": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "3m"
        }
      },
      "values": {
        "apiService": {
          "insecureSkipTLSVerify": false
        },
        "args": [
          "--kubelet-insecure-tls"
        ],
        "tls": {
          "type": "cert-manager"
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Helm install succeeded for release monitoring/metrics-server.v1 with chart metrics-server@3.13.0+457df0544ec2",
        "lastReconciled": "2025-11-01T23:31:10Z",
        "managedBy": "ResourceSet/flux-system/metrics-server"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:32:10Z",
          "message": "Helm install succeeded for release monitoring/metrics-server.v1 with chart metrics-server@3.13.0+457df0544ec2",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:32:10Z",
          "message": "Helm install succeeded for release monitoring/metrics-server.v1 with chart metrics-server@3.13.0+457df0544ec2",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Released"
        }
      ],
      "history": [
        {
          "appVersion": "0.8.0",
          "chartName": "metrics-server",
          "chartVersion": "3.13.0+457df0544ec2",
          "configDigest": "sha256:82b16ed00566b1bcb117ede6537fa4842acd9f6bbbe280b6adb3c1d0e45802c6",
          "digest": "sha256:309c88a5dd4e392c5830d4c0a3b6d7597bcb77afd40e4e94629e5c3b045f2715",
          "firstDeployed": "2025-11-01T23:31:42Z",
          "lastDeployed": "2025-11-01T23:31:42Z",
          "name": "metrics-server",
          "namespace": "monitoring",
          "ociDigest": "sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad",
          "status": "deployed",
          "version": 1
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "metrics-server",
          "namespace": "monitoring"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "system:metrics-server-aggregated-reader",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "system:metrics-server",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "metrics-server:system:auth-delegator",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "system:metrics-server",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "metrics-server-auth-reader",
          "namespace": "kube-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "metrics-server",
          "namespace": "monitoring"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "metrics-server",
          "namespace": "monitoring"
        },
        {
          "apiVersion": "apiregistration.k8s.io/v1",
          "kind": "APIService",
          "name": "v1beta1.metrics.k8s.io",
          "namespace": ""
        },
        {
          "apiVersion": "cert-manager.io/v1",
          "kind": "Certificate",
          "name": "metrics-server",
          "namespace": "monitoring"
        },
        {
          "apiVersion": "cert-manager.io/v1",
          "kind": "Issuer",
          "name": "metrics-server-issuer",
          "namespace": "monitoring"
        }
      ],
      "lastAttemptedConfigDigest": "sha256:82b16ed00566b1bcb117ede6537fa4842acd9f6bbbe280b6adb3c1d0e45802c6",
      "lastAttemptedGeneration": 1,
      "lastAttemptedReleaseAction": "install",
      "lastAttemptedReleaseActionDuration": "28.246828513s",
      "lastAttemptedRevision": "3.13.0+457df0544ec2",
      "lastAttemptedRevisionDigest": "sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "OCIRepository",
        "message": "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
        "name": "metrics-server",
        "namespace": "monitoring",
        "originRevision": "",
        "originURL": "https://github.com/kubernetes-sigs/metrics-server",
        "status": "Ready",
        "url": "oci://ghcr.io/controlplaneio-fluxcd/charts/metrics-server"
      },
      "storageNamespace": "monitoring"
    }
  },
  {
    "apiVersion": "helm.toolkit.fluxcd.io/v2",
    "kind": "HelmRelease",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "tailscale"
      },
      "name": "tailscale-operator",
      "namespace": "tailscale"
    },
    "spec": {
      "chart": {
        "spec": {
          "chart": "tailscale-operator",
          "interval": "24h",
          "reconcileStrategy": "ChartVersion",
          "sourceRef": {
            "kind": "HelmRepository",
            "name": "tailscale-operator"
          },
          "version": "*"
        }
      },
      "install": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "2m"
        }
      },
      "interval": "24h",
      "releaseName": "tailscale-operator",
      "upgrade": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "3m"
        }
      },
      "values": {
        "apiServerProxyConfig": {
          "allowImpersonation": "true",
          "mode": "false"
        },
        "operatorConfig": {
          "hostname": "homelab-operator"
        },
        "resources": {
          "limits": {
            "cpu": "2000m",
            "memory": "1Gi"
          },
          "requests": {
            "cpu": "100m",
            "memory": "128Mi"
          }
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Helm install succeeded for release tailscale/tailscale-operator.v1 with chart tailscale-operator@1.90.6",
        "lastReconciled": "2025-11-01T23:31:10Z",
        "managedBy": "ResourceSet/flux-system/tailscale-operator"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:10Z",
          "message": "Helm install succeeded for release tailscale/tailscale-operator.v1 with chart tailscale-operator@1.90.6",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:10Z",
          "message": "Helm install succeeded for release tailscale/tailscale-operator.v1 with chart tailscale-operator@1.90.6",
          "observedGeneration": 1,
          "reason": "InstallSucceeded",
          "status": "True",
          "type": "Released"
        }
      ],
      "helmChart": "tailscale/tailscale-tailscale-operator",
      "history": [
        {
          "appVersion": "v1.90.9",
          "chartName": "tailscale-operator",
          "chartVersion": "1.90.9",
          "configDigest": "sha256:ec864259c2bedeada53e194919f2416a1d6e742b4d5beb3555037ecce7c634d1",
          "digest": "sha256:f00bb59fbdb28284e525bd4d85aea707e728b89653f2acfa6d98cef3b93e28d5",
          "firstDeployed": "2025-11-01T23:31:01Z",
          "lastDeployed": "2025-11-26T19:26:23Z",
          "name": "tailscale-operator",
          "namespace": "tailscale",
          "status": "deployed",
          "version": 3
        },
        {
          "appVersion": "v1.90.8",
          "chartName": "tailscale-operator",
          "chartVersion": "1.90.8",
          "configDigest": "sha256:ec864259c2bedeada53e194919f2416a1d6e742b4d5beb3555037ecce7c634d1",
          "digest": "sha256:0d1e61ba399cbc6b8df9e625cb70009ec90a444d2293c280616884cee2dac132",
          "firstDeployed": "2025-11-01T23:31:01Z",
          "lastDeployed": "2025-11-19T22:22:04Z",
          "name": "tailscale-operator",
          "namespace": "tailscale",
          "status": "superseded",
          "version": 2
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "kube-apiserver-auth-proxy",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "operator",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "proxies",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "connectors.tailscale.com",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "dnsconfigs.tailscale.com",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "proxyclasses.tailscale.com",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "proxygroups.tailscale.com",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "recorders.tailscale.com",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "tailscale-auth-proxy",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "tailscale-operator",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "tailscale-auth-proxy",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "tailscale-operator",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "operator",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "Role",
          "name": "proxies",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "operator",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "RoleBinding",
          "name": "proxies",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "operator",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "IngressClass",
          "name": "tailscale",
          "namespace": ""
        }
      ],
      "lastAttemptedConfigDigest": "sha256:ec864259c2bedeada53e194919f2416a1d6e742b4d5beb3555037ecce7c634d1",
      "lastAttemptedGeneration": 1,
      "lastAttemptedReleaseAction": "install",
      "lastAttemptedReleaseActionDuration": "8.428667129s",
      "lastAttemptedRevision": "1.90.6",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "HelmRepository",
        "message": "stored artifact: revision 'sha256:578d082975ad264ba4d09368febb298c3beb7f18e459bb9d323d3b7c2fc4d475'",
        "name": "tailscale-operator",
        "namespace": "tailscale",
        "originRevision": "",
        "originURL": "",
        "status": "Ready",
        "url": "https://pkgs.tailscale.com/helmcharts"
      },
      "storageNamespace": "tailscale"
    }
  },
  {
    "apiVersion": "helm.toolkit.fluxcd.io/v2",
    "kind": "HelmRelease",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "zot"
      },
      "name": "zot-registry",
      "namespace": "registry"
    },
    "spec": {
      "chart": {
        "spec": {
          "chart": "zot",
          "interval": "24h",
          "reconcileStrategy": "ChartVersion",
          "sourceRef": {
            "kind": "HelmRepository",
            "name": "zot-registry"
          },
          "version": "*"
        }
      },
      "install": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "2m"
        }
      },
      "interval": "24h",
      "releaseName": "zot-registry",
      "upgrade": {
        "strategy": {
          "name": "RetryOnFailure",
          "retryInterval": "3m"
        }
      },
      "values": {
        "fullnameOverride": "zot-registry",
        "persistence": true,
        "pvc": {
          "create": true,
          "storage": "20Gi",
          "storageClassName": "standard"
        },
        "resources": {
          "limits": {
            "cpu": "2000m",
            "memory": "1Gi"
          },
          "requests": {
            "cpu": "100m",
            "memory": "128Mi"
          }
        },
        "service": {
          "port": 80,
          "type": "ClusterIP"
        },
        "strategy": {
          "type": "Recreate"
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Helm upgrade succeeded for release registry/zot-registry.v3 with chart zot@0.1.89",
        "lastReconciled": "2025-11-02T08:48:52Z",
        "managedBy": "ResourceSet/flux-system/zot-registry"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-02T08:48:52Z",
          "message": "Helm upgrade succeeded for release registry/zot-registry.v3 with chart zot@0.1.89",
          "observedGeneration": 2,
          "reason": "UpgradeSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-02T08:48:52Z",
          "message": "Helm upgrade succeeded for release registry/zot-registry.v3 with chart zot@0.1.89",
          "observedGeneration": 2,
          "reason": "UpgradeSucceeded",
          "status": "True",
          "type": "Released"
        }
      ],
      "helmChart": "registry/registry-zot-registry",
      "history": [
        {
          "appVersion": "v2.1.10",
          "chartName": "zot",
          "chartVersion": "0.1.89",
          "configDigest": "sha256:68a0cb4c346313fbf114884a6849aeff1c22d991dc7921b30d411f8471c38a09",
          "digest": "sha256:afeeb6dd28d7443a8675809da0a137b4957a392c704d8aa5c02bdb7e7ead9a29",
          "firstDeployed": "2025-11-01T23:31:41Z",
          "lastDeployed": "2025-11-02T08:48:38Z",
          "name": "zot-registry",
          "namespace": "registry",
          "status": "deployed",
          "version": 3
        },
        {
          "appVersion": "v2.1.10",
          "chartName": "zot",
          "chartVersion": "0.1.89",
          "configDigest": "sha256:68a0cb4c346313fbf114884a6849aeff1c22d991dc7921b30d411f8471c38a09",
          "digest": "sha256:6fc3a1dfff1edb1f5ca6887801223d713ab874335fe3253d066d47c524571d1b",
          "firstDeployed": "2025-11-01T23:31:41Z",
          "lastDeployed": "2025-11-02T08:45:34Z",
          "name": "zot-registry",
          "namespace": "registry",
          "status": "failed",
          "version": 2
        },
        {
          "appVersion": "v2.1.10",
          "chartName": "zot",
          "chartVersion": "0.1.89",
          "configDigest": "sha256:ddea6042f0d393e8cf3e920373afee22df11c35e730a7eee2519bed800bb2352",
          "digest": "sha256:da4ae11e60ef2fd825af7ff338374761f57a3c9442ace17a7a7f2929d8dd28da",
          "firstDeployed": "2025-11-01T23:31:41Z",
          "lastDeployed": "2025-11-01T23:31:41Z",
          "name": "zot-registry",
          "namespace": "registry",
          "status": "superseded",
          "version": 1
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "zot-registry",
          "namespace": "registry"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "zot-registry",
          "namespace": "registry"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "StatefulSet",
          "name": "zot-registry",
          "namespace": "registry"
        }
      ],
      "lastAttemptedConfigDigest": "sha256:68a0cb4c346313fbf114884a6849aeff1c22d991dc7921b30d411f8471c38a09",
      "lastAttemptedGeneration": 2,
      "lastAttemptedReleaseAction": "upgrade",
      "lastAttemptedReleaseActionDuration": "14.24459459s",
      "lastAttemptedRevision": "0.1.89",
      "observedGeneration": 2,
      "sourceRef": {
        "kind": "HelmRepository",
        "message": "stored artifact: revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
        "name": "zot-registry",
        "namespace": "registry",
        "originRevision": "",
        "originURL": "",
        "status": "Ready",
        "url": "https://zotregistry.dev/helm-charts"
      },
      "storageNamespace": "registry"
    }
  },
  {
    "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
    "kind": "Kustomization",
    "metadata": {
      "name": "cluster-infra",
      "namespace": "flux-system"
    },
    "spec": {
      "force": false,
      "healthCheckExprs": [
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "current": "status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == metadata.generation)",
          "kind": "ResourceSet"
        }
      ],
      "interval": "1h",
      "path": "infra",
      "prune": true,
      "retryInterval": "2m",
      "sourceRef": {
        "kind": "GitRepository",
        "name": "flux-system"
      },
      "timeout": "6m",
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
        "lastReconciled": "2025-11-18T11:09:21Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:09:21Z",
          "message": "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-18T11:09:21Z",
          "message": "Health check passed in 12.565167ms",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Healthy"
        }
      ],
      "history": [
        {
          "digest": "sha256:bbe7aa022b513c7ceb4cf38e9fee0cec579c96fee9bf15afcdeff34bf4eed934",
          "firstReconciled": "2025-11-06T21:36:41Z",
          "lastReconciled": "2025-11-18T11:09:21Z",
          "lastReconciledDuration": "76.890708ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "revision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff"
          },
          "totalReconciliations": 279
        },
        {
          "digest": "sha256:bbe7aa022b513c7ceb4cf38e9fee0cec579c96fee9bf15afcdeff34bf4eed934",
          "firstReconciled": "2025-11-11T13:27:39Z",
          "lastReconciled": "2025-11-11T14:15:40Z",
          "lastReconciledDuration": "6m0.068181246s",
          "lastReconciledStatus": "HealthCheckFailed",
          "metadata": {
            "revision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff"
          },
          "totalReconciliations": 7
        },
        {
          "digest": "sha256:b040c34218f94a8b600cb9686f3db56d931884fb7ce8bfc9355e4cb93f1e036c",
          "firstReconciled": "2025-11-06T21:35:43Z",
          "lastReconciled": "2025-11-06T21:35:43Z",
          "lastReconciledDuration": "5.222284961s",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "revision": "refs/heads/main@sha1:7ca2aaecb20927201120b33c3ad3f181294c5b98"
          },
          "totalReconciliations": 1
        },
        {
          "digest": "sha256:7dd5cdd3147fa75fecf07acc8a45d8fce39445af96bb0146c476795e7a2e3099",
          "firstReconciled": "2025-11-06T20:46:34Z",
          "lastReconciled": "2025-11-06T21:07:15Z",
          "lastReconciledDuration": "74.965459ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "revision": "refs/heads/main@sha1:d1bc4c4c6a8bf788fc776fdf2680ed91c6d0dd37"
          },
          "totalReconciliations": 2
        },
        {
          "digest": "sha256:75e8258dc921750cb17b10831c33931c6f1f6c418e081c8114303e1d0d6bc67f",
          "firstReconciled": "2025-11-03T18:48:35Z",
          "lastReconciled": "2025-11-06T20:08:46Z",
          "lastReconciledDuration": "84.788166ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "revision": "refs/heads/main@sha1:62a1e87e875328d999dd98090500182a26be833a"
          },
          "totalReconciliations": 74
        }
      ],
      "inventory": [
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "cert-manager",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "flux-status-server",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "metrics-server",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "tailscale-config",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "tailscale-operator",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "zot-registry",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSetInputProvider",
          "name": "flux-status-server",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      "lastAttemptedRevision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "GitRepository",
        "message": "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
        "name": "flux-system",
        "namespace": "flux-system",
        "originRevision": "",
        "originURL": "",
        "status": "Ready",
        "url": "https://github.com/stefanprodan/homelab.git"
      }
    }
  },
  {
    "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
    "kind": "Kustomization",
    "metadata": {
      "name": "flux-operator",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/instance": "flux-operator",
          "app.kubernetes.io/name": "flux-operator"
        }
      },
      "deletionPolicy": "Orphan",
      "force": true,
      "interval": "24h",
      "patches": [
        {
          "patch": "- op: replace\n  path: \"/spec/selector/matchLabels\"\n  value:\n    app.kubernetes.io/name: flux-operator\n    app.kubernetes.io/instance: flux-operator\n- op: replace\n  path: \"/spec/template/metadata/labels\"\n  value:\n    app.kubernetes.io/name: flux-operator\n    app.kubernetes.io/instance: flux-operator\n- op: add\n  path: \"/spec/template/spec/containers/0/env/-\"\n  value:\n    name: REPORTING_INTERVAL\n    value: \"30s\"",
          "target": {
            "kind": "Deployment"
          }
        }
      ],
      "path": "./flux-operator",
      "prune": true,
      "retryInterval": "5m",
      "serviceAccountName": "flux-operator",
      "sourceRef": {
        "kind": "OCIRepository",
        "name": "flux-operator"
      },
      "timeout": "5m",
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Applied revision: latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
        "lastReconciled": "2025-11-18T00:28:13Z",
        "managedBy": ""
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T00:28:13Z",
          "message": "Applied revision: latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-18T00:28:13Z",
          "message": "Health check passed in 17.192458ms",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Healthy"
        }
      ],
      "history": [
        {
          "digest": "sha256:cb9b9a44b9d94bd0bfd0cb8e3336e71841ccabf55da588ae1876548a77114759",
          "firstReconciled": "2025-11-01T23:31:13Z",
          "lastReconciled": "2025-11-18T00:28:13Z",
          "lastReconciledDuration": "139.516833ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "originRevision": "main@sha1:06e1897fcf773428155c5f6b9aadb5538169bb5c",
            "revision": "latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c"
          },
          "totalReconciliations": 19
        }
      ],
      "inventory": [
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "fluxinstances.fluxcd.controlplane.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "fluxreports.fluxcd.controlplane.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "resourcesetinputproviders.fluxcd.controlplane.io",
          "namespace": ""
        },
        {
          "apiVersion": "apiextensions.k8s.io/v1",
          "kind": "CustomResourceDefinition",
          "name": "resourcesets.fluxcd.controlplane.io",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "flux-operator-edit",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRole",
          "name": "flux-operator-view",
          "namespace": ""
        },
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "flux-operator-cluster-admin",
          "namespace": ""
        },
        {
          "apiVersion": "v1",
          "kind": "ServiceAccount",
          "name": "flux-operator",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "flux-operator",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "flux-operator",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedOriginRevision": "main@sha1:06e1897fcf773428155c5f6b9aadb5538169bb5c",
      "lastAppliedRevision": "latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
      "lastAttemptedRevision": "latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "OCIRepository",
        "message": "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
        "name": "flux-operator",
        "namespace": "flux-system",
        "originRevision": "main@sha1:06e1897fcf773428155c5f6b9aadb5538169bb5c",
        "originURL": "git://github.com/controlplaneio-fluxcd/flux-operator.git",
        "status": "Ready",
        "url": "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
      }
    }
  },
  {
    "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
    "kind": "Kustomization",
    "metadata": {
      "annotations": {
        "kustomize.toolkit.fluxcd.io/prune": "Disabled",
        "kustomize.toolkit.fluxcd.io/ssa": "Ignore"
      },
      "labels": {
        "app.kubernetes.io/instance": "flux-system",
        "app.kubernetes.io/managed-by": "flux-operator",
        "app.kubernetes.io/part-of": "flux",
        "app.kubernetes.io/version": "v2.7.3"
      },
      "name": "flux-system",
      "namespace": "flux-system"
    },
    "spec": {
      "force": false,
      "interval": "10m0s",
      "path": "./clusters/homelab",
      "prune": true,
      "sourceRef": {
        "kind": "GitRepository",
        "name": "flux-system"
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
        "lastReconciled": "2025-11-01T23:31:36Z",
        "managedBy": "FluxInstance/flux-system/flux"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:49:21Z",
          "message": "Applied revision: refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:92e6e90f2a0951f761fb9ed43f9814cc8c94b56666ca1a24cd0ecb7e85587331",
          "firstReconciled": "2025-11-01T23:30:57Z",
          "lastReconciled": "2025-11-18T11:49:21Z",
          "lastReconciledDuration": "32.727458ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "revision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff"
          },
          "totalReconciliations": 2406
        }
      ],
      "inventory": [
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "FluxInstance",
          "name": "flux",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "cluster-infra",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      "lastAttemptedRevision": "refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff",
      "lastHandledReconcileAt": "2025-11-06T23:35:16.275132+02:00",
      "observedGeneration": 1,
      "sourceRef": {
        "kind": "GitRepository",
        "message": "stored artifact for revision 'refs/heads/main@sha1:d676e33990dc2865d67c022d26dea93d5e3236ff'",
        "name": "flux-system",
        "namespace": "flux-system",
        "originRevision": "",
        "originURL": "",
        "status": "Ready",
        "url": "https://github.com/stefanprodan/homelab.git"
      }
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "OCIRepository",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "cert-manager"
      },
      "name": "cert-manager",
      "namespace": "cert-manager"
    },
    "spec": {
      "interval": "24h",
      "layerSelector": {
        "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
        "operation": "copy"
      },
      "provider": "generic",
      "ref": {
        "semver": "*"
      },
      "timeout": "60s",
      "url": "oci://quay.io/jetstack/charts/cert-manager"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
        "lastReconciled": "2025-11-01T23:31:03Z",
        "managedBy": "ResourceSet/flux-system/cert-manager"
      },
      "artifact": {
        "digest": "sha256:f72a727b1749df3521e7a65af5f18505f93b572d758841890270b150344b2b41",
        "lastUpdateTime": "2025-11-01T23:31:03Z",
        "metadata": {
          "artifacthub.io/category": "security",
          "artifacthub.io/license": "Apache-2.0",
          "artifacthub.io/prerelease": "false",
          "artifacthub.io/signKey": "fingerprint: 1020CF3C033D4F35BAE1C19E1226061C665DF13E\nurl: https://cert-manager.io/public-keys/cert-manager-keyring-2021-09-20-1020CF3C033D4F35BAE1C19E1226061C665DF13E.gpg\n",
          "org.opencontainers.image.authors": "cert-manager-maintainers (cert-manager-maintainers@googlegroups.com)",
          "org.opencontainers.image.created": "2025-10-15T16:29:51+01:00",
          "org.opencontainers.image.description": "A Helm chart for cert-manager",
          "org.opencontainers.image.source": "https://github.com/cert-manager/cert-manager",
          "org.opencontainers.image.title": "cert-manager",
          "org.opencontainers.image.url": "https://cert-manager.io",
          "org.opencontainers.image.version": "v1.19.1"
        },
        "path": "ocirepository/cert-manager/cert-manager/sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9.tar.gz",
        "revision": "v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9",
        "size": 140724,
        "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/cert-manager/cert-manager/sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:03Z",
          "message": "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:03Z",
          "message": "stored artifact for digest 'v1.19.1@sha256:9578566b26b2258bcb9a0be27feeaa7c0adaed635cc0f85b6293e42a80c58cc9'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedGeneration": 1,
      "observedLayerSelector": {
        "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
        "operation": "copy"
      },
      "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/cert-manager/cert-manager/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "OCIRepository",
    "metadata": {
      "name": "flux-operator",
      "namespace": "flux-system"
    },
    "spec": {
      "interval": "1h",
      "provider": "generic",
      "ref": {
        "tag": "latest"
      },
      "verify": {
        "provider": "cosign"
      },
      "timeout": "60s",
      "url": "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
        "lastReconciled": "2025-11-07T12:23:03Z",
        "managedBy": ""
      },
      "artifact": {
        "digest": "sha256:f3084e90b5b4c023321f43e5fee08b6e4b3a6a6d1938c614ae0ee91890fa249c",
        "lastUpdateTime": "2025-11-07T12:23:03Z",
        "metadata": {
          "org.opencontainers.image.created": "2025-11-07T12:12:32Z",
          "org.opencontainers.image.description": "Flux Operator",
          "org.opencontainers.image.revision": "main@sha1:06e1897fcf773428155c5f6b9aadb5538169bb5c",
          "org.opencontainers.image.source": "git://github.com/controlplaneio-fluxcd/flux-operator.git"
        },
        "path": "ocirepository/flux-system/flux-operator/sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c.tar.gz",
        "revision": "latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c",
        "size": 917016,
        "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/flux-system/flux-operator/sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-11T14:14:47Z",
          "message": "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-07T12:23:03Z",
          "message": "stored artifact for digest 'latest@sha256:043536cc6ec06ff978777ca31cf0adc3d654575a2aa8050988aadf90b9f9877c'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedGeneration": 1,
      "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/flux-system/flux-operator/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "OCIRepository",
    "metadata": {
      "labels": {
        "app.kubernetes.io/name": "metrics-server"
      },
      "name": "metrics-server",
      "namespace": "monitoring"
    },
    "spec": {
      "interval": "24h",
      "layerSelector": {
        "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
        "operation": "copy"
      },
      "provider": "generic",
      "ref": {
        "semver": "*"
      },
      "timeout": "60s",
      "url": "oci://ghcr.io/controlplaneio-fluxcd/charts/metrics-server"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
        "lastReconciled": "2025-11-01T23:31:42Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "artifact": {
        "digest": "sha256:a418dfdeb7b49a244cf6f848c4c3b7ee8eb6b0fdc19b0389cff98ceec4107a48",
        "lastUpdateTime": "2025-11-01T23:31:42Z",
        "metadata": {
          "artifacthub.io/changes": "- kind: added\n  description: \"Add chart options to secure the connection between Metrics Server and the Kubernetes API Server.\"\n- kind: added\n  description: \"Add `unhealthyPodEvictionPolicy` to the Metrics Server PDB as a user enabled feature.\"\n- kind: changed\n  description: \"Update the _Addon Resizer_ OCI image to [`1.8.23`](https://github.com/kubernetes/autoscaler/releases/tag/addon-resizer-1.8.23).\"\n- kind: changed\n  description: \"Update the _Metrics Server_ OCI image to [`0.8.0`](https://github.com/kubernetes-sigs/metrics-server/releases/tag/v0.8.0).\"\n",
          "org.opencontainers.image.authors": "stevehipwell, krmichel, endrec",
          "org.opencontainers.image.created": "2025-07-23T04:18:26Z",
          "org.opencontainers.image.description": "Metrics Server is a scalable, efficient source of container resource metrics for Kubernetes built-in autoscaling pipelines.",
          "org.opencontainers.image.source": "https://github.com/kubernetes-sigs/metrics-server",
          "org.opencontainers.image.title": "metrics-server",
          "org.opencontainers.image.url": "https://github.com/kubernetes-sigs/metrics-server",
          "org.opencontainers.image.version": "3.13.0"
        },
        "path": "ocirepository/monitoring/metrics-server/sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad.tar.gz",
        "revision": "3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad",
        "size": 13998,
        "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/monitoring/metrics-server/sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:42Z",
          "message": "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-01T23:31:42Z",
          "message": "stored artifact for digest '3.13.0@sha256:457df0544ec2553176bbaaba70bf5b68af6c400eff510a401b8eba1b13f9a8ad'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedGeneration": 1,
      "observedLayerSelector": {
        "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
        "operation": "copy"
      },
      "url": "http://source-controller.flux-system.svc.cluster.local./ocirepository/monitoring/metrics-server/latest.tar.gz"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "name": "cert-manager",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "cert-manager"
        }
      },
      "inputs": [
        {
          "namespace": "cert-manager",
          "version": "*"
        }
      ],
      "resources": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "metadata": {
            "name": "<< inputs.namespace >>"
          }
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "interval": "24h",
            "layerSelector": {
              "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
              "operation": "copy"
            },
            "ref": {
              "semver": "<< inputs.version | quote >>"
            },
            "url": "oci://quay.io/jetstack/charts/cert-manager"
          }
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "chartRef": {
              "kind": "OCIRepository",
              "name": "<< inputs.provider.name >>"
            },
            "install": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "2m"
              }
            },
            "interval": "24h",
            "releaseName": "<< inputs.provider.name >>",
            "upgrade": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "3m"
              }
            },
            "values": {
              "crds": {
                "enabled": true,
                "keep": false
              }
            }
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 35ms",
        "lastReconciled": "2025-11-18T11:52:14Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:52:14Z",
          "message": "Reconciliation finished in 35ms",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:7fa8d9d91bb4e05001d434630939607ab789ffbb8e34e567bb40fb315248c2f4",
          "firstReconciled": "2025-11-01T23:31:39Z",
          "lastReconciled": "2025-11-18T11:52:14Z",
          "lastReconciledDuration": "34.995209ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "3"
          },
          "totalReconciliations": 398
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "name": "cert-manager",
          "namespace": ""
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "name": "cert-manager",
          "namespace": "cert-manager"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "name": "cert-manager",
          "namespace": "cert-manager"
        }
      ],
      "lastAppliedRevision": "sha256:7fa8d9d91bb4e05001d434630939607ab789ffbb8e34e567bb40fb315248c2f4"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "7m"
      },
      "name": "cluster-infra",
      "namespace": "flux-system"
    },
    "spec": {
      "inputs": [
        {
          "source": "GitRepository"
        }
      ],
      "resources": [
        {
          "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
          "kind": "Kustomization",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "healthCheckExprs": [
              {
                "apiVersion": "fluxcd.controlplane.io/v1",
                "current": "status.conditions.filter(c, c.type == 'Ready').all(c, c.status == 'True' && c.observedGeneration == metadata.generation)",
                "kind": "ResourceSet"
              }
            ],
            "interval": "1h",
            "path": "infra",
            "prune": true,
            "retryInterval": "2m",
            "sourceRef": {
              "kind": "<< inputs.source >>",
              "name": "<< inputs.provider.namespace >>"
            },
            "timeout": "6m",
            "wait": true
          }
        }
      ]
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 27ms",
        "lastReconciled": "2025-11-18T11:50:16Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:50:16Z",
          "message": "Reconciliation finished in 27ms",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:3d97f8ccfa15bbfeae07f0b32c618a8a6ba97ac3d3a11c62bf41e1d8c05625a9",
          "firstReconciled": "2025-11-01T23:30:58Z",
          "lastReconciled": "2025-11-18T11:50:16Z",
          "lastReconciledDuration": "26.626ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "1"
          },
          "totalReconciliations": 397
        }
      ],
      "inventory": [
        {
          "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
          "kind": "Kustomization",
          "name": "cluster-infra",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "sha256:3d97f8ccfa15bbfeae07f0b32c618a8a6ba97ac3d3a11c62bf41e1d8c05625a9"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "labels": {
        "app.kubernetes.io/instance": "flux-operator",
        "app.kubernetes.io/name": "flux-operator"
      },
      "name": "flux-operator",
      "namespace": "flux-system"
    },
    "spec": {
      "inputs": [
        {
          "interval": "1h",
          "url": "oci://ghcr.io/controlplaneio-fluxcd/flux-operator-manifests"
        }
      ],
      "resources": [
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "interval": "<< inputs.interval | quote >>",
            "ref": {
              "tag": "latest"
            },
            "url": "<< inputs.url | quote >>"
          }
        },
        {
          "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
          "kind": "Kustomization",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "commonMetadata": {
              "labels": {
                "app.kubernetes.io/instance": "flux-operator",
                "app.kubernetes.io/name": "flux-operator"
              }
            },
            "deletionPolicy": "Orphan",
            "force": true,
            "interval": "24h",
            "patches": [
              {
                "patch": "- op: replace\n  path: \"/spec/selector/matchLabels\"\n  value:\n    app.kubernetes.io/name: flux-operator\n    app.kubernetes.io/instance: flux-operator\n- op: replace\n  path: \"/spec/template/metadata/labels\"\n  value:\n    app.kubernetes.io/name: flux-operator\n    app.kubernetes.io/instance: flux-operator\n- op: add\n  path: \"/spec/template/spec/containers/0/env/-\"\n  value:\n    name: REPORTING_INTERVAL\n    value: \"30s\"",
                "target": {
                  "kind": "Deployment"
                }
              }
            ],
            "path": "./flux-operator",
            "prune": true,
            "retryInterval": "5m",
            "serviceAccountName": "<< inputs.provider.name >>",
            "sourceRef": {
              "kind": "OCIRepository",
              "name": "<< inputs.provider.name >>"
            },
            "timeout": "5m",
            "wait": true
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 35ms",
        "lastReconciled": "2025-11-18T11:30:52Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:30:52Z",
          "message": "Reconciliation finished in 35ms",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:a324112e52314cc034cacda5c32d623d72fd952b2594149334d1823dd8df21ce",
          "firstReconciled": "2025-11-01T23:31:16Z",
          "lastReconciled": "2025-11-18T11:30:52Z",
          "lastReconciledDuration": "35.366792ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "2"
          },
          "totalReconciliations": 397
        },
        {
          "digest": "sha256:3a9e7101640ce2cbe3c61e846863386fb388d999e987476b756dd2a68d99f5f2",
          "firstReconciled": "2025-11-11T14:06:19Z",
          "lastReconciled": "2025-11-11T14:11:19Z",
          "lastReconciledDuration": "5m0.047161594s",
          "lastReconciledStatus": "ReconciliationFailed",
          "metadata": {
            "inputs": "1",
            "resources": "2"
          },
          "totalReconciliations": 2
        }
      ],
      "inventory": [
        {
          "apiVersion": "kustomize.toolkit.fluxcd.io/v1",
          "kind": "Kustomization",
          "name": "flux-operator",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "name": "flux-operator",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "sha256:a324112e52314cc034cacda5c32d623d72fd952b2594149334d1823dd8df21ce"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcile": "enabled",
        "fluxcd.controlplane.io/reconcileTimeout": "5m",
      },
      "name": "flux-status-server",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "flux-status-server"
        }
      },
      "dependsOn": [
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "name": "homelab-ingress",
          "ready": true,
          "readyExpr": "status.conditions.filter(c, c.type == 'ProxyGroupReady').all(c, c.status == 'True')"
        }
      ],
      "inputsFrom": [
        {
          "kind": "ResourceSetInputProvider",
          "name": "flux-status-server"
        }
      ],
      "resources": [
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "metadata": {
            "name": "flux-status-server",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "selector": {
              "matchLabels": {
                "app.kubernetes.io/name": "flux-status-server"
              }
            },
            "template": {
              "metadata": {
                "annotations": {
                  "prometheus.io/port": "9797",
                  "prometheus.io/scrape": "true"
                },
                "labels": {
                  "app.kubernetes.io/name": "flux-status-server"
                }
              },
              "spec": {
                "containers": [
                  {
                    "args": [
                      "--web-server-only",
                      "--web-server-port=9080"
                    ],
                    "image": "homelab-registry.tailbeb47.ts.net/flux-operator:<< inputs.tag >>@<< inputs.digest >>",
                    "imagePullPolicy": "IfNotPresent",
                    "livenessProbe": {
                      "httpGet": {
                        "path": "/healthz",
                        "port": 8081
                      }
                    },
                    "name": "server",
                    "ports": [
                      {
                        "containerPort": 9080,
                        "name": "http-web",
                        "protocol": "TCP"
                      },
                      {
                        "containerPort": 8080,
                        "name": "http-metrics",
                        "protocol": "TCP"
                      },
                      {
                        "containerPort": 8081,
                        "name": "http",
                        "protocol": "TCP"
                      }
                    ],
                    "readinessProbe": {
                      "httpGet": {
                        "path": "/readyz",
                        "port": 8081
                      }
                    },
                    "resources": {
                      "limits": {
                        "cpu": "2000m",
                        "memory": "512Mi"
                      },
                      "requests": {
                        "cpu": "100m",
                        "memory": "64Mi"
                      }
                    },
                    "securityContext": {
                      "allowPrivilegeEscalation": false,
                      "capabilities": {
                        "drop": [
                          "ALL"
                        ]
                      },
                      "readOnlyRootFilesystem": true,
                      "runAsNonRoot": true
                    },
                    "volumeMounts": [
                      {
                        "mountPath": "/data",
                        "name": "data"
                      }
                    ]
                  }
                ],
                "securityContext": {
                  "fsGroup": 65534,
                  "runAsNonRoot": true,
                  "runAsUser": 65534,
                  "seccompProfile": {
                    "type": "RuntimeDefault"
                  }
                },
                "serviceAccountName": "flux-operator",
                "volumes": [
                  {
                    "emptyDir": {},
                    "name": "data"
                  }
                ]
              }
            }
          }
        },
        {
          "apiVersion": "v1",
          "kind": "Service",
          "metadata": {
            "name": "flux-status-server",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "ports": [
              {
                "name": "http",
                "port": 9080,
                "protocol": "TCP",
                "targetPort": 9080
              }
            ],
            "selector": {
              "app.kubernetes.io/name": "flux-status-server"
            },
            "type": "ClusterIP"
          }
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "Ingress",
          "metadata": {
            "annotations": {
              "tailscale.com/proxy-group": "homelab-ingress"
            },
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "defaultBackend": {
              "service": {
                "name": "<< inputs.provider.name >>",
                "port": {
                  "number": 9080
                }
              }
            },
            "ingressClassName": "tailscale",
            "tls": [
              {
                "hosts": [
                  "homelab-status-page"
                ]
              }
            ]
          }
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "NetworkPolicy",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.provider.namespace >>"
          },
          "spec": {
            "ingress": [
              {
                "from": [
                  {
                    "namespaceSelector": {}
                  }
                ]
              }
            ],
            "podSelector": {
              "matchLabels": {
                "app.kubernetes.io/name": "<< inputs.provider.name >>"
              }
            },
            "policyTypes": [
              "Ingress"
            ]
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 50ms",
        "lastReconciled": "2025-11-18T11:39:39Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "inputProviderRefs":[
        {
          "type":"OCIArtifactTag",
          "name":"flux-status-server",
          "namespace":"flux-system"
        }
      ],
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:39:39Z",
          "message": "Reconciliation finished in 50ms",
          "observedGeneration": 5,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:5ffcfb1437cd080bdb2666161275b38461bb115c75117f6f40a5eb07347b989b",
          "firstReconciled": "2025-11-17T23:00:11Z",
          "lastReconciled": "2025-11-18T11:39:39Z",
          "lastReconciledDuration": "49.805625ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 14
        },
        {
          "digest": "sha256:20c7b71ba21561fb47538bb33bc3e5fbc57fadfe7ed87e24a6eeb29607ed6f52",
          "firstReconciled": "2025-11-17T11:14:42Z",
          "lastReconciled": "2025-11-17T22:35:06Z",
          "lastReconciledDuration": "51.145541ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 13
        },
        {
          "digest": "sha256:fa6bb5c1686a49103f2560c553dce70599dc7827a89b217031fbe063ea3e81a4",
          "firstReconciled": "2025-11-17T00:54:25Z",
          "lastReconciled": "2025-11-17T10:40:44Z",
          "lastReconciledDuration": "50.215125ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 11
        },
        {
          "digest": "sha256:3c34079b744ee4dd9f1cd6f6ecfba696d61d53016a2aae175955b4c1c7d3dc29",
          "firstReconciled": "2025-11-06T21:35:38Z",
          "lastReconciled": "2025-11-17T00:34:50Z",
          "lastReconciledDuration": "54.078459ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 246
        },
        {
          "digest": "sha256:9d5d73e9806e24c731cfac9bddfa40c2f14bd26b983ff9c08b7bc2a38f3cc503",
          "firstReconciled": "2025-11-06T21:29:37Z",
          "lastReconciled": "2025-11-06T21:33:01Z",
          "lastReconciledDuration": "29.65225ms",
          "lastReconciledStatus": "ReconciliationFailed",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 10
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "Service",
          "name": "flux-status-server",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "apps/v1",
          "kind": "Deployment",
          "name": "flux-status-server",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "Ingress",
          "name": "flux-status-server",
          "namespace": "flux-system"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "NetworkPolicy",
          "name": "flux-status-server",
          "namespace": "flux-system"
        }
      ],
      "lastAppliedRevision": "sha256:5ffcfb1437cd080bdb2666161275b38461bb115c75117f6f40a5eb07347b989b",
      "lastHandledReconcileAt": "2025-11-06T23:33:00.823725+02:00"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "name": "metrics-server",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "metrics-server"
        }
      },
      "dependsOn": [
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "cert-manager",
          "namespace": "flux-system",
          "ready": true
        }
      ],
      "inputs": [
        {
          "namespace": "monitoring",
          "version": "*"
        }
      ],
      "resources": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "metadata": {
            "name": "<< inputs.namespace >>"
          }
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "interval": "24h",
            "layerSelector": {
              "mediaType": "application/vnd.cncf.helm.chart.content.v1.tar+gzip",
              "operation": "copy"
            },
            "ref": {
              "semver": "<< inputs.version | quote >>"
            },
            "url": "oci://ghcr.io/controlplaneio-fluxcd/charts/metrics-server"
          }
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "chartRef": {
              "kind": "OCIRepository",
              "name": "<< inputs.provider.name >>"
            },
            "install": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "2m"
              }
            },
            "interval": "24h",
            "releaseName": "<< inputs.provider.name >>",
            "upgrade": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "3m"
              }
            },
            "values": {
              "apiService": {
                "insecureSkipTLSVerify": false
              },
              "args": [
                "--kubelet-insecure-tls"
              ],
              "tls": {
                "type": "cert-manager"
              }
            }
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 36ms",
        "lastReconciled": "2025-11-18T10:56:54Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T10:56:54Z",
          "message": "Reconciliation finished in 36ms",
          "observedGeneration": 1,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:a2e6b4e28249995ae6a7e25b687b39f6b18101c1405391466a0cba1054c58603",
          "firstReconciled": "2025-11-01T23:32:15Z",
          "lastReconciled": "2025-11-18T10:56:54Z",
          "lastReconciledDuration": "36.074834ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "3"
          },
          "totalReconciliations": 396
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "name": "monitoring",
          "namespace": ""
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "name": "metrics-server",
          "namespace": "monitoring"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "OCIRepository",
          "name": "metrics-server",
          "namespace": "monitoring"
        }
      ],
      "lastAppliedRevision": "sha256:a2e6b4e28249995ae6a7e25b687b39f6b18101c1405391466a0cba1054c58603"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "name": "tailscale-config",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "tailscale"
        }
      },
      "dependsOn": [
        {
          "apiVersion": "fluxcd.controlplane.io/v1",
          "kind": "ResourceSet",
          "name": "tailscale-operator",
          "namespace": "flux-system",
          "ready": true
        }
      ],
      "inputs": [
        {
          "hostnamePrefix": "homelab",
          "user": "stefanprodan@github"
        }
      ],
      "resources": [
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "metadata": {
            "name": "homelab-cluster-admin"
          },
          "roleRef": {
            "apiGroup": "rbac.authorization.k8s.io",
            "kind": "ClusterRole",
            "name": "cluster-admin"
          },
          "subjects": [
            {
              "apiGroup": "rbac.authorization.k8s.io",
              "kind": "User",
              "name": "<< inputs.user >>"
            }
          ]
        },
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "metadata": {
            "name": "<< inputs.hostnamePrefix >>-cluster"
          },
          "spec": {
            "kubeAPIServer": {
              "mode": "auth"
            },
            "replicas": 1,
            "tags": [
              "tag:k8s"
            ],
            "type": "kube-apiserver"
          }
        },
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "metadata": {
            "name": "<< inputs.hostnamePrefix >>-ingress"
          },
          "spec": {
            "replicas": 1,
            "tags": [
              "tag:k8s"
            ],
            "type": "ingress"
          }
        }
      ]
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 31ms",
        "lastReconciled": "2025-11-18T11:25:10Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:25:10Z",
          "message": "Reconciliation finished in 31ms",
          "observedGeneration": 2,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:2ba728f62df1f679a2d472c9fb24bf46daffe0f80129c30fc334f5fe3169619f",
          "firstReconciled": "2025-11-01T23:31:19Z",
          "lastReconciled": "2025-11-18T11:25:10Z",
          "lastReconciledDuration": "30.827166ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "3"
          },
          "totalReconciliations": 398
        }
      ],
      "inventory": [
        {
          "apiVersion": "rbac.authorization.k8s.io/v1",
          "kind": "ClusterRoleBinding",
          "name": "homelab-cluster-admin",
          "namespace": ""
        },
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "name": "homelab-cluster",
          "namespace": ""
        },
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "name": "homelab-ingress",
          "namespace": ""
        }
      ],
      "lastAppliedRevision": "sha256:2ba728f62df1f679a2d472c9fb24bf46daffe0f80129c30fc334f5fe3169619f"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "name": "tailscale-operator",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "tailscale"
        }
      },
      "inputs": [
        {
          "hostnamePrefix": "homelab",
          "namespace": "tailscale",
          "version": "*"
        }
      ],
      "resources": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "metadata": {
            "name": "<< inputs.namespace >>"
          }
        },
        {
          "apiVersion": "v1",
          "kind": "Secret",
          "metadata": {
            "annotations": {
              "fluxcd.controlplane.io/copyFrom": "flux-system/tailscale-oauth"
            },
            "name": "operator-oauth",
            "namespace": "<< inputs.namespace >>"
          }
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "HelmRepository",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "interval": "24h",
            "url": "https://pkgs.tailscale.com/helmcharts"
          }
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "chart": {
              "spec": {
                "chart": "tailscale-operator",
                "interval": "24h",
                "sourceRef": {
                  "kind": "HelmRepository",
                  "name": "<< inputs.provider.name >>"
                },
                "version": "*"
              }
            },
            "install": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "2m"
              }
            },
            "interval": "24h",
            "releaseName": "<< inputs.provider.name >>",
            "upgrade": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "3m"
              }
            },
            "values": {
              "apiServerProxyConfig": {
                "allowImpersonation": "true",
                "mode": "false"
              },
              "operatorConfig": {
                "hostname": "<< inputs.hostnamePrefix >>-operator"
              },
              "resources": {
                "limits": {
                  "cpu": "2000m",
                  "memory": "1Gi"
                },
                "requests": {
                  "cpu": "100m",
                  "memory": "128Mi"
                }
              }
            }
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 43ms",
        "lastReconciled": "2025-11-18T11:52:24Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:52:24Z",
          "message": "Reconciliation finished in 43ms",
          "observedGeneration": 2,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:35c7432bd251f8eec9a71f0937ac33c9ca086702e340992c56a1a977a65cfeae",
          "firstReconciled": "2025-11-01T23:31:14Z",
          "lastReconciled": "2025-11-18T11:52:24Z",
          "lastReconciledDuration": "43.119459ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 398
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "name": "tailscale",
          "namespace": ""
        },
        {
          "apiVersion": "v1",
          "kind": "Secret",
          "name": "operator-oauth",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "name": "tailscale-operator",
          "namespace": "tailscale"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "HelmRepository",
          "name": "tailscale-operator",
          "namespace": "tailscale"
        }
      ],
      "lastAppliedRevision": "sha256:35c7432bd251f8eec9a71f0937ac33c9ca086702e340992c56a1a977a65cfeae"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSet",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileTimeout": "5m"
      },
      "name": "zot-registry",
      "namespace": "flux-system"
    },
    "spec": {
      "commonMetadata": {
        "labels": {
          "app.kubernetes.io/name": "zot"
        }
      },
      "dependsOn": [
        {
          "apiVersion": "tailscale.com/v1alpha1",
          "kind": "ProxyGroup",
          "name": "homelab-ingress",
          "ready": true,
          "readyExpr": "status.conditions.filter(c, c.type == 'ProxyGroupReady').all(c, c.status == 'True')"
        }
      ],
      "inputs": [
        {
          "hostnamePrefix": "homelab",
          "namespace": "registry",
          "version": "*"
        }
      ],
      "resources": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "metadata": {
            "name": "<< inputs.namespace >>"
          }
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "HelmRepository",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "interval": "24h",
            "url": "https://zotregistry.dev/helm-charts"
          }
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "metadata": {
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "chart": {
              "spec": {
                "chart": "zot",
                "interval": "24h",
                "sourceRef": {
                  "kind": "HelmRepository",
                  "name": "<< inputs.provider.name >>"
                },
                "version": "*"
              }
            },
            "install": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "2m"
              }
            },
            "interval": "24h",
            "releaseName": "<< inputs.provider.name >>",
            "upgrade": {
              "strategy": {
                "name": "RetryOnFailure",
                "retryInterval": "3m"
              }
            },
            "values": {
              "fullnameOverride": "<< inputs.provider.name >>",
              "persistence": true,
              "pvc": {
                "create": true,
                "storage": "20Gi",
                "storageClassName": "standard"
              },
              "resources": {
                "limits": {
                  "cpu": "2000m",
                  "memory": "1Gi"
                },
                "requests": {
                  "cpu": "100m",
                  "memory": "128Mi"
                }
              },
              "service": {
                "port": 80,
                "type": "ClusterIP"
              },
              "strategy": {
                "type": "Recreate"
              }
            }
          }
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "Ingress",
          "metadata": {
            "annotations": {
              "tailscale.com/proxy-group": "<< inputs.hostnamePrefix >>-ingress"
            },
            "name": "<< inputs.provider.name >>",
            "namespace": "<< inputs.namespace >>"
          },
          "spec": {
            "defaultBackend": {
              "service": {
                "name": "<< inputs.provider.name >>",
                "port": {
                  "number": 80
                }
              }
            },
            "ingressClassName": "tailscale",
            "tls": [
              {
                "hosts": [
                  "<< inputs.hostnamePrefix >>-registry"
                ]
              }
            ]
          }
        }
      ],
      "wait": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 41ms",
        "lastReconciled": "2025-11-18T10:54:54Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T10:54:54Z",
          "message": "Reconciliation finished in 41ms",
          "observedGeneration": 3,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "history": [
        {
          "digest": "sha256:b08adacc97ecc5b817ea654e7f091944304caca48a6e40334423b6a3dc416fc9",
          "firstReconciled": "2025-11-02T08:48:54Z",
          "lastReconciled": "2025-11-18T10:54:54Z",
          "lastReconciledDuration": "41.419084ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 388
        },
        {
          "digest": "sha256:c260cf7082af1c618deaba00548d3c135969f2aa4a63b643698ca4c01588084b",
          "firstReconciled": "2025-11-01T23:32:20Z",
          "lastReconciled": "2025-11-02T08:34:57Z",
          "lastReconciledDuration": "39.261875ms",
          "lastReconciledStatus": "ReconciliationSucceeded",
          "metadata": {
            "inputs": "1",
            "resources": "4"
          },
          "totalReconciliations": 10
        }
      ],
      "inventory": [
        {
          "apiVersion": "v1",
          "kind": "Namespace",
          "name": "registry",
          "namespace": ""
        },
        {
          "apiVersion": "helm.toolkit.fluxcd.io/v2",
          "kind": "HelmRelease",
          "name": "zot-registry",
          "namespace": "registry"
        },
        {
          "apiVersion": "networking.k8s.io/v1",
          "kind": "Ingress",
          "name": "zot-registry",
          "namespace": "registry"
        },
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "HelmRepository",
          "name": "zot-registry",
          "namespace": "registry"
        }
      ],
      "lastAppliedRevision": "sha256:b08adacc97ecc5b817ea654e7f091944304caca48a6e40334423b6a3dc416fc9"
    }
  },
  {
    "apiVersion": "fluxcd.controlplane.io/v1",
    "kind": "ResourceSetInputProvider",
    "metadata": {
      "annotations": {
        "fluxcd.controlplane.io/reconcileEvery": "5m",
      },
      "name": "flux-status-server",
      "namespace": "flux-system"
    },
    "spec": {
      "filter": {
        "includeTag": "web",
        "limit": 1
      },
      "type": "OCIArtifactTag",
      "url": "oci://homelab-registry.tailbeb47.ts.net/flux-operator"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Reconciliation finished in 207ms",
        "lastReconciled": "2025-11-18T12:15:29Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:15:29Z",
          "message": "Reconciliation finished in 207ms",
          "observedGeneration": 2,
          "reason": "ReconciliationSucceeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "exportedInputs": [
        {
          "digest": "sha256:2af20e4de759d1edf881a2ce1c6d2fd8eec830d2f9d43421ec49c2bf7b89975f",
          "id": "43254079",
          "tag": "web"
        }
      ],
      "lastExportedRevision": "sha256:f9680e09616a9d6a36d4e09538a039b1ee5a5c44871196c64b7b05ef45a9c9a0",
      "lastHandledReconcileAt": "2025-11-04T11:31:53.473116+02:00"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "name": "aws-configs",
      "namespace": "flux-system"
    },
    "spec": {
      "bucketName": "flux-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "5m0s",
      "provider": "aws",
      "region": "us-east-1"
    },
    "status": {
      "reconcilerRef": {
        "status": "Failed",
        "message": "authentication failed:\nSTS: AssumeRoleWithWebIdentity, https response error\nPost \"https://sts.arn.amazonaws.com/\": dial tcp: lookupts.arn.amazonaws.com on 10.100.0.10:53: no such host",
        "lastReconciled": "2025-11-18T10:30:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T10:30:00Z",
          "message": "authentication failed:\nSTS: AssumeRoleWithWebIdentity, https response error\nPost \"https://sts.arn.amazonaws.com/\": dial tcp: lookupts.arn.amazonaws.com on 10.100.0.10:53: no such host",
          "observedGeneration": 1,
          "reason": "AuthenticationFailed",
          "status": "False",
          "type": "Ready"
        }
      ],
      "observedGeneration": 1
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "name": "prod-configs",
      "namespace": "flux-system"
    },
    "spec": {
      "bucketName": "prod-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "10m0s",
      "provider": "aws",
      "region": "us-west-2"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
        "lastReconciled": "2025-11-18T08:30:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "artifact": {
        "digest": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "lastUpdateTime": "2025-11-18T08:30:00Z",
        "path": "bucket/flux-system/prod-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz",
        "revision": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "size": 2048,
        "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/prod-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T08:30:00Z",
          "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-18T08:30:00Z",
          "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedGeneration": 1,
      "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/prod-configs/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "name": "dev-configs",
      "namespace": "flux-system"
    },
    "spec": {
      "bucketName": "dev-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "5m0s",
      "provider": "aws",
      "region": "us-west-2"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
        "lastReconciled": "2025-11-18T11:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "artifact": {
        "digest": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "lastUpdateTime": "2025-11-18T11:00:00Z",
        "path": "bucket/flux-system/dev-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz",
        "revision": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "size": 1024,
        "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/dev-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:00:00Z",
          "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        },
        {
          "lastTransitionTime": "2025-11-18T11:00:00Z",
          "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "ArtifactInStorage"
        }
      ],
      "observedGeneration": 1,
      "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/dev-configs/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "name": "staging-configs",
      "namespace": "flux-system"
    },
    "spec": {
      "bucketName": "staging-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "5m0s",
      "provider": "aws",
      "region": "eu-west-1"
    },
    "status": {
      "reconcilerRef": {
        "status": "Progressing",
        "message": "reconciliation in progress: fetching artifact",
        "lastReconciled": "2025-11-18T11:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T11:00:00Z",
          "message": "reconciliation in progress: fetching artifact",
          "observedGeneration": 1,
          "reason": "Progressing",
          "status": "Unknown",
          "type": "Ready"
        }
      ],
      "observedGeneration": 1
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "name": "preview-configs",
      "namespace": "flux-system",
      "annotations": {
        "fluxcd.controlplane.io/suspendedBy": "John Doe"
      }
    },
    "spec": {
      "bucketName": "preview-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "10m0s",
      "provider": "aws",
      "region": "us-east-1",
      "suspend": true
    },
    "status": {
      "reconcilerRef": {
        "status": "Suspended",
        "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "artifact": {
        "digest": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "lastUpdateTime": "2025-11-18T07:00:00Z",
        "path": "bucket/flux-system/preview-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz",
        "revision": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        "size": 512,
        "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/preview-configs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T09:00:00Z",
          "message": "stored artifact for revision 'sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "observedGeneration": 1,
      "url": "http://source-controller.flux-system.svc.cluster.local./bucket/flux-system/preview-configs/latest.tar.gz"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "Bucket",
    "metadata": {
      "annotations": {
        "kustomize.toolkit.fluxcd.io/ssa": "Ignore"
      },
      "name": "default-configs",
      "namespace": "default"
    },
    "spec": {
      "bucketName": "default-configs",
      "endpoint": "s3.amazonaws.com",
      "interval": "15m0s",
      "provider": "generic"
    },
    "status": {
      "reconcilerRef": {
        "status": "Unknown",
        "message": "No status information available",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "observedGeneration": -1
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1beta3",
    "kind": "Alert",
    "metadata": {
      "name": "slack",
      "namespace": "flux-system"
    },
    "spec": {
      "eventSeverity": "info",
      "eventSources": [
        {
          "kind": "GitRepository",
          "name": "*"
        },
        {
          "kind": "Kustomization",
          "name": "*"
        },
        {
          "kind": "HelmRelease",
          "name": "*"
        }
      ],
      "providerRef": {
        "name": "slack"
      },
      "suspend": false
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Valid configuration",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      }
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1beta3",
    "kind": "Provider",
    "metadata": {
      "name": "slack",
      "namespace": "flux-system"
    },
    "spec": {
      "address": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
      "channel": "#flux-notifications",
      "secretRef": {
        "name": "slack-webhook-url"
      },
      "type": "slack",
      "username": "Flux"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Valid configuration",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      }
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1",
    "kind": "Receiver",
    "metadata": {
      "name": "github-webhook",
      "namespace": "flux-system"
    },
    "spec": {
      "events": [
        "push"
      ],
      "resources": [
        {
          "apiVersion": "source.toolkit.fluxcd.io/v1",
          "kind": "GitRepository",
          "name": "flux-system"
        }
      ],
      "secretRef": {
        "name": "github-webhook-token"
      },
      "type": "github"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Receiver initialized for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b",
        "lastReconciled": "2025-11-18T10:20:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T10:20:00Z",
          "message": "Receiver initialized for path: /hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "observedGeneration": 1,
      "webhookPath": "/hook/bed6d00b5555b1603e1f59b94d7fdbca58089cb5663633fb83f2815dc626d92b"
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1beta3",
    "kind": "Alert",
    "metadata": {
      "name": "msteams",
      "namespace": "automation"
    },
    "spec": {
      "eventSeverity": "info",
      "eventSources": [
        {
          "kind": "GitRepository",
          "name": "*"
        }
      ],
      "providerRef": {
        "name": "msteams"
      },
      "suspend": false
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Valid configuration",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      }
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1beta3",
    "kind": "Provider",
    "metadata": {
      "name": "msteams",
      "namespace": "automation"
    },
    "spec": {
      "address": "https://outlook.office.com/webhook/YOUR/TEAMS/WEBHOOK",
      "secretRef": {
        "name": "msteams-webhook-url"
      },
      "type": "msteams"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Valid configuration",
        "lastReconciled": "2025-11-18T09:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      }
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "HelmRepository",
    "metadata": {
      "name": "zot-registry",
      "namespace": "registry"
    },
    "spec": {
      "interval": "24h0m0s",
      "url": "https://zotregistry.dev/helm-charts"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
        "lastReconciled": "2025-11-01T23:31:41Z",
        "managedBy": "ResourceSet/flux-system/zot-registry"
      },
      "artifact": {
        "digest": "sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125",
        "lastUpdateTime": "2025-11-01T23:31:41Z",
        "path": "helmrepository/registry/zot-registry/index.yaml",
        "revision": "sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125",
        "size": 1024,
        "url": "http://source-controller.flux-system.svc.cluster.local./helmrepository/registry/zot-registry/index.yaml"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:41Z",
          "message": "stored artifact for revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "HelmRepository",
    "metadata": {
      "name": "tailscale-operator",
      "namespace": "tailscale"
    },
    "spec": {
      "interval": "24h0m0s",
      "url": "https://pkgs.tailscale.com/helmcharts"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'sha256:2b1fdd97e969c82ee149a7ee8b00f55061760832f23c39a3235936f0912f2125'",
        "lastReconciled": "2025-11-01T23:31:02Z",
        "managedBy": "ResourceSet/flux-system/tailscale-operator"
      },
      "artifact": {
        "digest": "sha256:578d082975ad264ba4d09368febb298c3beb7f18e459bb9d323d3b7c2fc4d475",
        "lastUpdateTime": "2025-11-01T23:31:02Z",
        "path": "helmrepository/tailscale/tailscale-operator/index.yaml",
        "revision": "sha256:578d082975ad264ba4d09368febb298c3beb7f18e459bb9d323d3b7c2fc4d475",
        "size": 2048,
        "url": "http://source-controller.flux-system.svc.cluster.local./helmrepository/tailscale/tailscale-operator/index.yaml"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-01T23:31:02Z",
          "message": "stored artifact for revision 'sha256:578d082975ad264ba4d09368febb298c3beb7f18e459bb9d323d3b7c2fc4d475'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "GitRepository",
    "metadata": {
      "name": "podinfo",
      "namespace": "automation"
    },
    "spec": {
      "interval": "1m0s",
      "ref": {
        "branch": "main"
      },
      "verify": {
        "mode": "HEAD"
      },
      "url": "https://github.com/stefanprodan/podinfo"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "stored artifact for revision 'main@sha1:c1b613a1e083a8918185b11b317f3c75e3c1b6d0'",
        "lastReconciled": "2025-11-18T12:00:00Z",
        "managedBy": "Kustomization/flux-system/apps"
      },
      "artifact": {
        "digest": "sha256:9c224393021c31a3ce372812b0eaf81085e5a633c50115b79c1e3f72e21a6b8f",
        "lastUpdateTime": "2025-11-18T12:00:00Z",
        "path": "gitrepository/automation/podinfo/c1b613a1e083a8918185b11b317f3c75e3c1b6d0.tar.gz",
        "revision": "main@sha1:c1b613a1e083a8918185b11b317f3c75e3c1b6d0",
        "size": 12345,
        "url": "http://source-controller.flux-system.svc.cluster.local./gitrepository/automation/podinfo/c1b613a1e083a8918185b11b317f3c75e3c1b6d0.tar.gz"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:00:00Z",
          "message": "stored artifact for revision 'main@sha1:c1b613a1e083a8918185b11b317f3c75e3c1b6d0'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "image.toolkit.fluxcd.io/v1",
    "kind": "ImageRepository",
    "metadata": {
      "name": "podinfo",
      "namespace": "automation"
    },
    "spec": {
      "image": "ghcr.io/stefanprodan/podinfo",
      "interval": "5m0s"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "successful scan, found 211 tags",
        "lastReconciled": "2025-11-18T12:05:00Z",
        "managedBy": "Kustomization/flux-system/apps"
      },
      "canonicalImageName": "ghcr.io/stefanprodan/podinfo",
      "lastScanResult": {
        "latestTags": [
          "6.2.0",
          "6.1.0"
        ],
        "scanTime": "2025-11-18T12:05:00Z",
        "tagCount": 2
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:05:00Z",
          "message": "successful scan, found 211 tags",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "image.toolkit.fluxcd.io/v1",
    "kind": "ImagePolicy",
    "metadata": {
      "name": "podinfo",
      "namespace": "automation"
    },
    "spec": {
      "imageRepositoryRef": {
        "name": "podinfo"
      },
      "policy": {
        "semver": {
          "range": ">=6.0.0"
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 6.2.0",
        "lastReconciled": "2025-11-18T12:10:00Z",
        "managedBy": "Kustomization/flux-system/apps"
      },
      "latestImage": "ghcr.io/stefanprodan/podinfo:6.2.0",
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:10:00Z",
          "message": "Latest image tag for 'ghcr.io/stefanprodan/podinfo' resolved to 6.2.0",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "image.toolkit.fluxcd.io/v1",
    "kind": "ImageUpdateAutomation",
    "metadata": {
      "name": "podinfo",
      "namespace": "automation"
    },
    "spec": {
      "git": {
        "checkout": {
          "ref": {
            "branch": "main"
          }
        },
        "commit": {
          "author": {
            "email": "fluxcdbot@users.noreply.github.com",
            "name": "fluxcdbot"
          },
        },
        "push": {
          "branch": "main"
        }
      },
      "interval": "15m0s",
      "sourceRef": {
        "kind": "GitRepository",
        "name": "podinfo"
      },
      "update": {
        "path": "./kustomize",
        "strategy": "Setters"
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "pushed commit '3ebb95c' to branch 'main'",
        "lastReconciled": "2025-11-18T12:15:00Z",
        "managedBy": "Kustomization/flux-system/apps"
      },
      "lastAutomationRunTime": "2025-11-18T12:15:00Z",
      "lastPushCommit": "c1b613a1e083a8918185b11b317f3c75e3c1b6d0",
      "lastPushTime": "2025-11-18T12:15:00Z",
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:15:00Z",
          "message": "pushed commit '3ebb95c' to branch 'main'",
          "observedGeneration": 1,
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "notification.toolkit.fluxcd.io/v1",
    "kind": "Receiver",
    "metadata": {
      "name": "podinfo-webhook",
      "namespace": "automation"
    },
    "spec": {
      "type": "github",
      "events": [
        "push"
      ],
      "secretRef": {
        "name": "webhook-secret"
      },
      "resources": [
        {
          "kind": "GitRepository",
          "name": "podinfo"
        }
      ]
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Receiver initialized for path: /hook/cbdee599b7977a520a36692e5b872c39d09ee53dd75b2e3ae117fea283958fbf",
        "lastReconciled": "2025-11-18T12:00:00Z",
        "managedBy": "Kustomization/flux-system/apps"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:00:00Z",
          "message": "Receiver initialized for path: /hook/cbdee599b7977a520a36692e5b872c39d09ee53dd75b2e3ae117fea283958fbf",
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ],
      "url": "/hook/cbdee599b7977a520a36692e5b872c39d09ee53dd75b2e3ae117fea283958fbf",
      "observedGeneration": 1,
      "lastHandledReconcileAt": "2025-11-18T12:00:00Z"
    }
  },
  {
    "apiVersion": "source.toolkit.fluxcd.io/v1",
    "kind": "HelmRepository",
    "metadata": {
      "name": "bitnami",
      "namespace": "flux-system"
    },
    "spec": {
      "interval": "24h",
      "url": "https://charts.bitnami.com/bitnami"
    },
    "status": {
      "reconcilerRef": {
        "status": "Failed",
        "message": "failed to fetch index: unable to connect to the server\nGet \"https://charts.bitnami.com/bitnami/index.yaml\": timeout awaiting response headers",
        "lastReconciled": "2025-11-18T12:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:00:00Z",
          "message": "failed to fetch index: unable to connect to the server\nGet \"https://charts.bitnami.com/bitnami/index.yaml\": timeout awaiting response headers",
          "reason": "Failed",
          "status": "False",
          "type": "Ready"
        }
      ],
      "observedGeneration": 1
    }
  },
  {
    "apiVersion": "image.toolkit.fluxcd.io/v1",
    "kind": "ImageRepository",
    "metadata": {
      "name": "redis",
      "namespace": "automation"
    },
    "spec": {
      "suspend": true,
      "image": "redis",
      "interval": "10m0s"
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "successful scan, found 50 tags",
        "lastReconciled": "2025-11-18T12:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "canonicalImageName": "redis",
      "lastScanResult": {
        "latestTags": [
          "7.0.5",
          "6.2.7"
        ],
        "scanTime": "2025-11-18T12:00:00Z",
        "tagCount": 50
      },
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:00:00Z",
          "message": "successful scan, found 50 tags",
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  },
  {
    "apiVersion": "image.toolkit.fluxcd.io/v1",
    "kind": "ImagePolicy",
    "metadata": {
      "name": "redis",
      "namespace": "automation"
    },
    "spec": {
      "suspend": true,
      "imageRepositoryRef": {
        "name": "redis"
      },
      "policy": {
        "semver": {
          "range": ">=6.0.0"
        }
      }
    },
    "status": {
      "reconcilerRef": {
        "status": "Ready",
        "message": "Latest image tag for 'redis' resolved to 7.0.5",
        "lastReconciled": "2025-11-18T12:00:00Z",
        "managedBy": "Kustomization/flux-system/flux-system"
      },
      "latestImage": "redis:7.0.5",
      "conditions": [
        {
          "lastTransitionTime": "2025-11-18T12:00:00Z",
          "message": "Latest image tag for 'redis' resolved to 7.0.5",
          "reason": "Succeeded",
          "status": "True",
          "type": "Ready"
        }
      ]
    }
  }
]

// Export function that returns the appropriate mock resource based on query parameters
export const getMockResource = (endpoint) => {
  // Parse query params from endpoint URL
  // eslint-disable-next-line no-undef
  const url = new URL(endpoint, 'http://localhost')
  const params = url.searchParams

  const kind = params.get('kind')
  const name = params.get('name')
  const namespace = params.get('namespace')

  // Search for matching resource in the array
  const resource = mockResourcesArray.find(r =>
    r.kind === kind &&
    r.metadata.name === name &&
    r.metadata.namespace === namespace
  )

  if (!resource) {
    return null
  }

  // Inject userActions field - array of allowed actions (empty for Bucket)
  const userActions = kind !== 'Bucket' ? ['reconcile', 'suspend', 'resume'] : []
  return {
    ...resource,
    status: {
      ...resource.status,
      userActions
    }
  }
}
