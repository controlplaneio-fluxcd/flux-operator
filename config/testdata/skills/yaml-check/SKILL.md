---
name: yaml-check
description: Run yq to validate and lint YAML files for correctness
---

# YAML Check

Use [yq](https://github.com/mikefarah/yq) to validate and inspect YAML files.

## Usage

To validate a single YAML file:

```bash
yq eval '.' file.yaml > /dev/null
```

To validate all YAML files in a directory:

```bash
./scripts/validation.sh /path/to/dir
```

To pretty-print a YAML file:

```bash
yq eval '.' file.yaml
```

## Common Options

- `-e EXPRESSION` — evaluate a yq expression
- `-i` — edit file in-place
- `-o json` — output as JSON
- `-P` — pretty-print output
