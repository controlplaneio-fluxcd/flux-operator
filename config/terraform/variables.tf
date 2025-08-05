variable "flux_version" {
  description = "Flux version semver range"
  type        = string
  default     = "2.x"
}

variable "flux_registry" {
  description = "Flux distribution registry"
  type        = string
  default     = "ghcr.io/fluxcd"
}

variable "cluster_type" {
  description = "Cluster type, e.g. kubernetes, openshift, azure, aws, gcp"
  type        = string
  default     = "kubernetes"
}

variable "cluster_size" {
  description = "Cluster size, e.g. small, medium, large"
  type        = string
  default     = "medium"
}

variable "git_token" {
  description = "Git PAT"
  sensitive   = true
  type        = string
  default     = ""
}

variable "github_app_id" {
  description = "GitHub App ID"
  type        = string
  default     = ""
}

variable "github_app_installation_id" {
  description = "GitHub App Installation ID"
  type        = string
  default     = ""
}

variable "github_app_pem" {
  description = "The contents of the GitHub App private key PEM file"
  sensitive   = true
  type        = string
  default     = ""
}

variable "git_url" {
  description = "Git repository URL"
  type        = string
  nullable    = false
}

variable "git_path" {
  description = "Path to the cluster manifests in the Git repository"
  type        = string
  nullable    = false
}

variable "git_ref" {
  description = "Git branch or tag in the format refs/heads/main or refs/tags/v1.0.0"
  type        = string
  default     = "refs/heads/main"
}
