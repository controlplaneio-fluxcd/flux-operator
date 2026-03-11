// Copyright 2026 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package v1

import (
	"fmt"
	"hash/adler32"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AgentCatalogKind is the kind of the Agent Catalog API.
	AgentCatalogKind = "Catalog"

	// AgentCatalogVerifyProviderCosign is the cosign verification provider.
	AgentCatalogVerifyProviderCosign = "cosign"
)

// AgentCatalog is the Agent skills catalog.
type AgentCatalog struct {
	metav1.TypeMeta `json:",inline"`

	// Spec holds the catalog configuration.
	// +required
	Spec AgentCatalogSpec `json:"spec"`

	// Status holds the catalog status.
	// +optional
	Status AgentCatalogStatus `json:"status,omitempty"`
}

// AgentCatalogSpec holds the catalog configuration.
type AgentCatalogSpec struct {
	// Sources is the list of OCI repositories providing skills.
	// +optional
	Sources []AgentCatalogSource `json:"sources,omitempty"`
}

// AgentCatalogSource holds the configuration for an OCI skills source.
type AgentCatalogSource struct {
	// Repository is the OCI repository URL.
	// +required
	Repository string `json:"repository"`

	// Tag is the OCI artifact tag.
	// +required
	Tag string `json:"tag"`

	// Verify holds the signature verification configuration.
	// +optional
	Verify *AgentCatalogVerify `json:"verify,omitempty"`

	// TargetAgents is the list of agent IDs for which skill symlinks are managed.
	// +optional
	TargetAgents []string `json:"targetAgents,omitempty"`
}

// AgentCatalogVerify holds the signature verification configuration.
type AgentCatalogVerify struct {
	// Provider is the verification provider (e.g. "cosign").
	// +required
	Provider string `json:"provider"`

	// MatchOIDCIdentity is the list of OIDC identity matchers.
	// +optional
	MatchOIDCIdentity []OIDCIdentity `json:"matchOIDCIdentity,omitempty"`
}

// OIDCIdentity holds the OIDC issuer and subject for verification.
type OIDCIdentity struct {
	// Issuer is the OIDC issuer URL.
	// +required
	Issuer string `json:"issuer"`

	// Subject is the OIDC subject regexp.
	// +required
	Subject string `json:"subject"`
}

// AgentCatalogStatus holds the catalog status.
type AgentCatalogStatus struct {
	// Inventory is the list of installed skill sources.
	// +optional
	Inventory []AgentCatalogInventoryEntry `json:"inventory,omitempty"`
}

// AgentCatalogInventoryEntry holds the status of an installed skill source.
type AgentCatalogInventoryEntry struct {
	// ID is the Adler-32 checksum of the source repository URL.
	// +required
	ID string `json:"id"`

	// URL is the OCI artifact URL including tag.
	// +required
	URL string `json:"url"`

	// Digest is the OCI artifact digest.
	// +required
	Digest string `json:"digest"`

	// LastUpdateAt is the timestamp of the last update.
	// +required
	LastUpdateAt string `json:"lastUpdateAt"`

	// Annotations holds the OCI artifact manifest annotations.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Skills is the list of skills provided by this source.
	// +optional
	Skills []AgentCatalogSkill `json:"skills,omitempty"`
}

// FindSource returns the source matching the given repository and its index,
// or nil and -1 if not found.
func (s *AgentCatalogSpec) FindSource(repo string) (*AgentCatalogSource, int) {
	for i := range s.Sources {
		if s.Sources[i].Repository == repo {
			return &s.Sources[i], i
		}
	}
	return nil, -1
}

// SkillNames returns a slice of skill names from the inventory entry.
func (e *AgentCatalogInventoryEntry) SkillNames() []string {
	names := make([]string, len(e.Skills))
	for i, s := range e.Skills {
		names[i] = s.Name
	}
	return names
}

// RepositoryID returns the Adler-32 checksum of a repository URL as a hex string.
func RepositoryID(repo string) string {
	return fmt.Sprintf("%08x", adler32.Checksum([]byte(repo)))
}

// FindInventoryEntry returns the inventory entry matching the given repository
// and its index, or -1 if not found.
func (s *AgentCatalogStatus) FindInventoryEntry(repo string) (*AgentCatalogInventoryEntry, int) {
	id := RepositoryID(repo)
	for i := range s.Inventory {
		if s.Inventory[i].ID == id {
			return &s.Inventory[i], i
		}
	}
	return nil, -1
}

// AgentCatalogSkill holds the metadata of an installed skill.
type AgentCatalogSkill struct {
	// Name is the skill name.
	// +required
	Name string `json:"name"`

	// Checksum is the directory hash of the installed skill contents.
	// +required
	Checksum string `json:"checksum"`
}
