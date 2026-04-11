variable "bootstrap_revision" {
  description = "Bump to trigger a new bootstrap run."
  type        = number
  default     = 1
  nullable    = false
}

variable "cluster_name" {
  description = "Name of the cluster directory under clusters/ (e.g. staging)."
  type        = string
  nullable    = false
}

variable "cluster_region" {
  description = "Cloud provider region where the cluster runs (e.g. eu-west-2)."
  type        = string
  nullable    = false
}

variable "git_token" {
  description = "Git PAT for HTTPS authentication (e.g. for GitLab). Can be omitted for public repositories or when using a GitHub App."
  sensitive   = true
  type        = string
  default     = ""
}

variable "github_app_id" {
  description = "GitHub App ID."
  type        = string
  default     = ""
}

variable "github_app_installation_owner" {
  description = "GitHub App Installation Owner."
  type        = string
  default     = ""
}

variable "github_app_pem" {
  description = "The contents of the GitHub App private key PEM file."
  sensitive   = true
  type        = string
  default     = ""
}
