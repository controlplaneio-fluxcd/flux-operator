---
name: shell-check
description: Run shellcheck to lint shell scripts for common issues and best practices
---

# Shell Check

Use [shellcheck](https://www.shellcheck.net/) to analyze shell scripts
for syntax errors, semantic problems, and style issues.

## Usage

To check a single script:

```bash
shellcheck script.sh
```

To check all shell scripts in the current directory:

```bash
shellcheck *.sh
```

To check scripts recursively:

```bash
find . -name '*.sh' -exec shellcheck {} +
```

## Common Options

- `-e CODE` — exclude a specific warning (e.g. `shellcheck -e SC2086 script.sh`)
- `-s SHELL` — specify the shell dialect (`bash`, `sh`, `dash`, `ksh`)
- `-f FORMAT` — output format (`tty`, `json`, `gcc`, `diff`)
