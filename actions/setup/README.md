# Setup Flux Operator CLI GitHub Action

This GitHub Action can be used to install the Flux Operator CLI on GitHub runners for usage in workflows.
All GitHub runners are supported, including Ubuntu, Windows, and macOS.

## Usage

Example workflow for printing the latest version:

```yaml
name: Check the latest version

on:
  workflow_dispatch:

jobs:
  check-latest-flux-operator-version:
    runs-on: ubuntu-latest
    steps:
      - name: Setup Flux Operator CLI
        uses: controlplaneio-fluxcd/flux-operator/actions/setup@main
        with:
          version: latest
      - name: Print Flux Operator Version
        run: flux-operator version --client
```

## Action Inputs

| Name               | Description                      | Default                   |
|--------------------|----------------------------------|---------------------------|
| `version`          | Flux Operator version            | The latest stable release |
| `bindir`           | Alternative location for the CLI | `$RUNNER_TOOL_CACHE`      |

## Action Outputs

| Name               | Description                                                  |
|--------------------|--------------------------------------------------------------|
| `version`          | The Flux Operator CLI version that was effectively installed |
