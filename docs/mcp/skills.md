---
title: Flux MCP Server Skills
description: Flux agent skills served by the Flux MCP Server with the MCP Skills extension
---

# Flux MCP Server Skills

The Flux MCP Server serves the [GitOps Agent Skills](https://github.com/fluxcd/agent-skills)
as MCP resources. The skills are embedded in the `flux-operator-mcp` binary, and each
Flux Operator release ships the latest `fluxcd/agent-skills` release available at release time.

The served skills are:

- **gitops-knowledge**: answers questions about Flux CD and generates YAML for all Flux custom resources.
- **gitops-cluster-debug**: debugs and troubleshoots Flux on live Kubernetes clusters using the Flux MCP Server tools.
- **gitops-repo-audit**: audits Flux GitOps repositories for structure, security and operational best practices.

## Skill resources

Every file of a skill is exposed as an MCP resource under the `skill://` URI scheme,
with the URI form `skill://<skill-name>/<file-path>`. For example:

| Resource                                                     | MIME type            |
|--------------------------------------------------------------|----------------------|
| `skill://gitops-cluster-debug/SKILL.md`                      | `text/markdown`      |
| `skill://gitops-cluster-debug/references/troubleshooting.md` | `text/markdown`      |
| `skill://gitops-repo-audit/scripts/validate.sh`              | `text/x-shellscript` |

The skill files are listed by `resources/list` and read with `resources/read`.
Relative references in a `SKILL.md`, such as `references/flux-crds.md`, resolve against the skill root, e.g. `skill://gitops-cluster-debug/references/flux-crds.md`.

The server instructions list the URI of each skill's `SKILL.md` with a short summary.

## MCP Skills extension

The server implements the [MCP Skills extension](https://modelcontextprotocol.io/extensions/skills/overview)
(`io.modelcontextprotocol/skills`) and declares it in the server capabilities together with the `resources` capability:

```json
{
  "capabilities": {
    "resources": {},
    "tools": {},
    "extensions": {
      "io.modelcontextprotocol/skills": {
        "directoryRead": true
      }
    }
  }
}
```

The extension adds the following methods:

- `skills/list` returns an entry for each skill, with pagination.
- `skills/get` returns the entry of a single skill given the URI of its `SKILL.md`.
- `resources/directory/read` lists the direct children of a skill directory, such as
  `skill://gitops-knowledge` or `skill://gitops-knowledge/references`. Subdirectories
  are listed with the `inode/directory` MIME type.

A skill entry contains the URI of the `SKILL.md`, its YAML frontmatter as a JSON object,
and a complete manifest of the skill files with the SHA-256 digest and size of each file:

```json
{
  "uri": "skill://gitops-knowledge/SKILL.md",
  "frontmatter": {
    "name": "gitops-knowledge",
    "description": "Flux CD and Flux Operator expert ...",
    "license": "Apache-2.0"
  },
  "resources": [
    {
      "uri": "skill://gitops-knowledge/SKILL.md",
      "digest": "sha256:...",
      "size": 15544
    }
  ]
}
```

The digests and sizes match the bytes returned by `resources/read`.
The results carry a one hour cache hint (`ttlMs`) with the `public` cache scope.

Requests for an unknown skill, file or directory fail with the JSON-RPC error `-32602` (Invalid params).

## Updating the skills

The served skills change only when the Flux MCP Server is upgraded.
To use a newer release of the skills, install them locally with the [Flux Operator CLI](cli.md):

```shell
flux-operator skills install ghcr.io/fluxcd/agent-skills --agent claude-code
```
