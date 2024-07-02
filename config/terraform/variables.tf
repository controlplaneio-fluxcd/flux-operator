variable "git_token" {
  description = "Git PAT"
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
