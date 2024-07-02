terraform {
  required_version = ">= 1.7"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = ">= 2.27"
    }
    helm = {
      source  = "hashicorp/helm"
      version = ">= 2.12"
    }
  }
}

// Create the  flux-system namespace.
resource "kubernetes_namespace" "flux_system" {
  metadata {
    name = "flux-system"
  }

  lifecycle {
    ignore_changes = [metadata]
  }
}

// Create a Kubernetes secret with the Git credentials
// if a Git token is provided.
resource "kubernetes_secret" "git_auth" {
  count      = var.git_token != "" ? 1 : 0
  depends_on = [kubernetes_namespace.flux_system]

  metadata {
    name      = "flux-system"
    namespace = "flux-system"
  }

  data = {
    username = "git"
    password = var.git_token
  }

  type = "Opaque"
}

// Install the Flux Operator.
resource "helm_release" "flux_operator" {
  depends_on = [kubernetes_namespace.flux_system]

  name       = "flux-operator"
  namespace  = "flux-system"
  repository = "oci://ghcr.io/controlplaneio-fluxcd/charts"
  chart      = "flux-operator"
  wait       = true
}

// Configure the Flux instance.
resource "helm_release" "flux_instance" {
  depends_on = [helm_release.flux_operator]

  name       = "flux"
  namespace  = "flux-system"
  repository = "oci://ghcr.io/controlplaneio-fluxcd/charts"
  chart      = "flux-instance"

  // Configure the Flux components and kustomize patches.
  values = [
    file("values/components.yaml")
  ]

  // Configure the Flux distribution.
  set {
    name  = "instance.distribution.version"
    value = var.flux_version
  }
  set {
    name  = "instance.distribution.registry"
    value = var.flux_registry
  }

  // Configure Flux Git sync.
  set {
    name  = "instance.sync.kind"
    value = "GitRepository"
  }
  set {
    name  = "instance.sync.url"
    value = var.git_url
  }
  set {
    name  = "instance.sync.path"
    value = var.git_path
  }
  set {
    name  = "instance.sync.ref"
    value = var.git_ref
  }
  set {
    name  = "instance.sync.pullSecret"
    value = var.git_token != "" ? "flux-system" : ""
  }
}
