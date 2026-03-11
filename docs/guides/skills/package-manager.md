---
title: Package Manager for AI Agent Skills
description: Using Flux Operator's CLI to manage Agent skills as OCI artifacts.
---

# Package Manager for AI Agent Skills

Flux Operator's [CLI](cli.md) can be used to manage Agent Skills as cryptographically signed OCI artifacts.
This allows organizations to package, sign, distribute, and deploy skills across teams
with confidence in their integrity and authenticity.

Unlike Git-based skill distribution (e.g. `npx skills`) where consumers implicitly trust
the source repository with no built-in integrity verification, the OCI approach provides
content-addressable digests, Sigstore keyless signing, and tampering detection.

Organizations consuming skills from public repositories should review them
for prompt injection and malicious scripts before internal distribution.
Once vetted, skills can be published to an internal OCI-compliant container
registry and signed with the organization's own identity, giving teams
a trusted source independent of upstream repositories.

## Skill Format

Each skill is a directory containing a `SKILL.md` file with YAML frontmatter
that declares the skill's `name` and `description`, followed by Markdown instructions.
Skills must conform to the [Agent Skills Standard](https://agentskills.io/).

A skill directory may also contain additional files such as scripts and references.
For example:

```text
my-skill/
├── SKILL.md          # Required: instructions + metadata
├── scripts/          # Optional: executable code
├── references/       # Optional: documentation
└── assets/           # Optional: templates, schemas
```

Where `SKILL.md` contains:

```text
---
name: my-skill
description: What the skill does and when to use it
license: Apache-2.0
allowed-tools: Read Grep Glob Bash(scripts/my-script.sh:*)
compatibility: Scripts require awk, git, flux-operator CLI
---

# My Skill

Instructions for the AI agent on how to use this skill.
```

Skill names must be lowercase alphanumeric with hyphens, must not start or end
with a hyphen, must not contain consecutive hyphens, and may not exceed 64 characters.

## Skills Distribution

### Git Repository

Skills are authored in a Git repository with each skill in its own directory.
For example, the [fluxcd/agent-skills](https://github.com/fluxcd/agent-skills) repository:

```text
fluxcd/agent-skills/
├── skills/
│   ├── gitops-cluster-debug/
│   │   ├── SKILL.md
│   │   ├── assets/
│   │   ├── evals/
│   │   └── references/
│   └── gitops-repo-audit/
│       ├── SKILL.md
│       ├── assets/
│       ├── evals/
│       ├── references/
│       └── scripts/
└── .github/workflows/
    └── publish.yml
```

### Registry Authentication

The `flux-operator` CLI uses the standard Docker configuration for registry authentication.
It automatically looks for credentials in `~/.docker/config.json`
and `DOCKER_CONFIG` environment variable.

Alternatively, if you are running in a CI environment like GitHub Actions,
the `docker/login-action` will automatically set up the credentials for the CLI.

### Publish Workflow

Publish skills to a container registry with one or more tags:

```shell
flux-operator skills publish ghcr.io/my-org/agent-skills \
  --tag v1.0.0,latest \
  --sign
```

By default, the publish command looks for skills in the `skills/` directory
relative to the current working directory. Git metadata (source URL, revision,
version, creation timestamp) is automatically captured as OCI annotations during publish.

The `--sign` flag signs the artifact using
[Sigstore cosign](https://docs.sigstore.dev/cosign/signing/overview/) keyless signing
(Fulcio certificate + Rekor transparency log). This requires an OIDC token,
which is available automatically in GitHub Actions and other CI environments.

Running the command locally will trigger a browser-based OIDC authentication flow to obtain the token.
Note that the `cosign` CLI must be installed and present in the system PATH for signing to work.

#### GitHub Actions Workflow

To publish skills automatically on push to the main branch or for Git tags,
here is an example based on the
[fluxcd/agent-skills](https://github.com/fluxcd/agent-skills) workflow:

```yaml
name: publish
on:
  push:
    branches: [main]
    tags: ['*']

jobs:
  skills-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read 
      packages: write  # for pushing to GHCR
      id-token: write  # for signing with cosign
    steps:
      - name: Checkout
        uses: actions/checkout@v6
      - name: Install flux-operator CLI
        uses: controlplaneio-fluxcd/flux-operator@main
      - name: Install cosign CLI
        uses: sigstore/cosign-installer@v4
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Prepare tags
        id: prep
        run: |
          TAG=${{ github.ref_name }}
          VERSION="${{ github.ref_name }}-${GITHUB_SHA::8}"
          if [[ $GITHUB_REF == refs/tags/* ]]; then
            TAG=latest
            VERSION="${{ github.ref_name }}"
          fi
          echo "tag=${TAG}" >> $GITHUB_OUTPUT
          echo "version=${VERSION}" >> $GITHUB_OUTPUT
      - name: Publish skills
        run: |
          flux-operator skills publish ghcr.io/${{ github.repository }} \
            --path ./skills \
            --tag ${{ steps.prep.outputs.version }} \
            --tag ${{ steps.prep.outputs.tag }} \
            --diff-tag ${{ steps.prep.outputs.tag }} \
            -a 'org.opencontainers.image.description=AI Agent Skills for Flux CD' \
            -a 'org.opencontainers.image.licenses=Apache-2.0' \
            --sign
```

When pushing to a branch (e.g. `main`), the artifact is tagged with both
`main-<short-sha>` (immutable) and `main` (mutable), and `--diff-tag main`
skips the push if the skills content hasn't changed since the last `main` push.

When pushing a Git tag (e.g. `v1.0.0`), the artifact is tagged with both
`v1.0.0` (immutable) and `latest` (mutable).

## OCI Artifact Structure

Skills are packaged as OCI artifacts with the following media types:

| Component | Media Type |
|-----------|-----------|
| Config | `application/vnd.cncf.flux.config.v1+json` |
| Content layer | `application/vnd.cncf.flux.content.v1.tar+gzip` |

The content layer is a tar+gzip archive with normalized headers (zero uid/gid,
zero timestamps) ensuring reproducible builds — the same source content always
produces the same archive bytes, enabling reliable content-based diffing.

OCI manifest annotations are auto-populated from Git metadata:

| Annotation | Source |
|-----------|--------|
| `org.opencontainers.image.created` | Last commit timestamp |
| `org.opencontainers.image.source` | Remote origin URL (normalized to HTTPS) |
| `org.opencontainers.image.revision` | `<ref>@sha1:<commit>` |
| `org.opencontainers.image.version` | Exact semver tag on HEAD (if any) |

User-provided annotations (via `-a`) take precedence over auto-populated values.

## Skills Lifecycle Management

### Installation

Skills are installed to the `.agents/skills/` directory relative to the
current working directory. Each skill gets its own subdirectory
(e.g. `.agents/skills/my-skill/`).

Install skills from a ghcr.io repository with default tag `latest` and
automatic signature verification:

```shell
flux-operator skills install ghcr.io/my-org/agent-skills
```

For ghcr.io repositories, the OIDC issuer and subject regex are derived
automatically from the registry path. The issuer defaults to
`https://token.actions.githubusercontent.com` and the subject regex is
derived from the repository owner (e.g., `^https://github\.com/my-org/.*$`).

Install a specific version:

```shell
flux-operator skills install ghcr.io/my-org/agent-skills --tag v1.0.0
```

For non-ghcr.io registries, you must provide OIDC verification flags explicitly:

```shell
flux-operator skills install docker.io/my-org/skills \
  --verify-oidc-issuer=https://token.actions.githubusercontent.com \
  --verify-oidc-subject-regex='^https://github\.com/my-org/.*$'
```

For air-gapped environments, use offline verification with a Sigstore trusted root:

```shell
flux-operator skills install ghcr.io/my-org/agent-skills \
  --verify-trusted-root /path/to/trusted_root.json
```

Skip verification entirely (not recommended for production):

```shell
flux-operator skills install ghcr.io/my-org/agent-skills --verify=false
```

#### Agent Symlinks

By default, skills are installed to `.agents/skills/` which is the universal
skills directory. Many AI agents look for skills in agent-specific directories
(e.g. `.claude/skills/`, `.github/skills`, `.kiro/skills/`).

The `--agent` flag creates per-skill symlinks from agent-specific
directories to the canonical location:

```shell
flux-operator skills install ghcr.io/my-org/agent-skills \
  --agent claude-code --agent kiro
```

This creates relative symlinks like:

```text
.claude/skills/my-skill -> ../../.agents/skills/my-skill
.kiro/skills/my-skill   -> ../../.agents/skills/my-skill
```

The agent configuration is stored per-source in `catalog.yaml`, so each OCI source
can target different agents. The `--agent universal` value (the default) stores skills
in `.agents/skills/` without creating additional symlinks.

When re-installing with different agents, old symlinks are automatically cleaned up.
Uninstalling a source removes all associated agent symlinks and cleans up empty directories.

The installation process performs the following steps:

1. Resolve the remote digest for the tag
2. Verify the cosign signature using the digest reference
3. Pull the artifact content by digest
4. Discover skills in the artifact (each must have a valid `SKILL.md`)
5. Check for name conflicts with skills from other sources
6. Sync skill directories to `.agents/skills/`
7. Write `catalog.yaml` and `catalog-lock.yaml`
8. Create agent-specific symlinks if `--agent` is set

### Updates

Update all installed skills to the latest remote version:

```shell
flux-operator skills update
```

The update command checks each source in `catalog.yaml`, compares the remote digest
against the local lock, and pulls new content when changes are detected.
If skills have been tampered with or deleted locally (detected via checksum comparison),
they are automatically restored from the upstream artifact.

Use `--dry-run` to check for available updates without applying them.
The command exits with code 1 if updates are available, making it suitable for CI checks:

```shell
flux-operator skills update --dry-run
```

### Uninstallation

Remove all skills installed from a specific repository:

```shell
flux-operator skills uninstall ghcr.io/my-org/agent-skills
```

This removes the skill directories, the source entry from `catalog.yaml`,
and the inventory entry from `catalog-lock.yaml`.
Symlinks for any agents are also removed,
and empty agent directories are cleaned up.

To remove all skills from all repositories:

```shell
flux-operator skills uninstall --all
```

### Listing

List all installed skills with their source, version, and digest:

```shell
flux-operator skills list
```

The output displays a table with the following columns:

| Name | Repository | Tag | Digest | Last Update |
|------|-----------|-----|--------|-------------|
| gitops-cluster-debug | ghcr.io/fluxcd/agent-skills | latest | sha256:abc1234abc... | 2026-03-10T12:00:00Z |
| gitops-repo-audit | ghcr.io/fluxcd/agent-skills | latest | sha256:abc1234abc... | 2026-03-10T12:00:00Z |

## Catalog Files

The skills catalog is stored as two files in the `.agents/skills/` directory:

- **`catalog.yaml`** — the spec file containing the list of OCI sources with their
  verification settings. This is the user-editable, authoritative source of truth.
  Commit this file to version control.
- **`catalog-lock.yaml`** — the auto-generated lock file containing the full inventory
  with resolved digests, per-skill checksums, artifact annotations, and timestamps.
  This file is similar to `go.sum` or `package-lock.json`. Commit it to version control
  to enable reproducible installs and drift detection.

### catalog.yaml

```yaml
apiVersion: agent.fluxcd.controlplane.io/v1
kind: Catalog
spec:
  sources:
  - repository: ghcr.io/fluxcd/agent-skills
    tag: latest
    targetAgents:
    - claude-code
    - kiro
    verify:
      provider: cosign
      matchOIDCIdentity:
      - issuer: https://token.actions.githubusercontent.com
        subject: ^https://github\.com/fluxcd/.*$
```

### catalog-lock.yaml

```yaml
# This manifest was generated by flux-operator. DO NOT EDIT.
apiVersion: agent.fluxcd.controlplane.io/v1
kind: Catalog
spec:
  sources:
  - repository: ghcr.io/fluxcd/agent-skills
    tag: latest
    targetAgents:
    - claude-code
    - kiro
    verify:
      provider: cosign
      matchOIDCIdentity:
      - issuer: https://token.actions.githubusercontent.com
        subject: ^https://github\.com/fluxcd/.*$
status:
  inventory:
  - id: 7d5f09ce
    url: ghcr.io/fluxcd/agent-skills:latest
    digest: sha256:ca93bbd5e0605821b0e3b2e8fd41df5a0280b9482437092ba1c585632196d2b1
    lastUpdateAt: "2026-03-10T10:26:18Z"
    annotations:
      org.opencontainers.image.created: "2026-03-09T22:27:39+02:00"
      org.opencontainers.image.revision: refs/heads/main@sha1:90e873c8e11cb9e08cd99241383168d8cc2c4e89
      org.opencontainers.image.source: https://github.com/fluxcd/agent-skills
    skills:
    - name: gitops-cluster-debug
      checksum: h1:XNNWGBTaK0j1zkeuytR/n8mvQfjnXZmlr9Ig7SbMitE=
    - name: gitops-repo-audit
      checksum: h1:2zdcnzL2ll8BfSFk4r/H0CW0diMcDexABBrcdvlES6c=
```

The `id` field is an Adler-32 checksum of the repository URL, used to match inventory
entries to their source. The `checksum` on each skill is a directory hash of the installed
skill contents, used for tamper detection during updates.

## Multiple Sources

Skills can be installed from multiple OCI repositories. Each skill name must be unique
across all sources — the CLI rejects installations that would create name conflicts.

```shell
flux-operator skills install ghcr.io/fluxcd/agent-skills
flux-operator skills install ghcr.io/my-org/skills --tag v2.0.0
```

Both sources are tracked in `catalog.yaml`, and `skills update` checks all sources
for new versions:

```yaml
apiVersion: agent.fluxcd.controlplane.io/v1
kind: Catalog
spec:
  sources:
  - repository: ghcr.io/fluxcd/agent-skills
    tag: latest
    verify:
      provider: cosign
      matchOIDCIdentity:
      - issuer: https://token.actions.githubusercontent.com
        subject: ^https://github\.com/fluxcd/.*$
  - repository: ghcr.io/my-org/skills
    tag: v2.0.0
    verify:
      provider: cosign
      matchOIDCIdentity:
      - issuer: https://token.actions.githubusercontent.com
        subject: ^https://github\.com/my-org/.*$
```
