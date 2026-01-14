---
title: Configuration
description: Configuring Flux Operator with flags and environment variables
---

# Flux Operator Configuration

The Flux Operator can be configured using command-line flags and environment variables.

## Flags

The following flags are available:

| Flag | Default | Description |
|------|---------|-------------|
| `--concurrent` | `10` | The number of concurrent resource reconciles |
| `--default-service-account` | | Default service account used for impersonation |
| `--default-workload-identity-service-account` | | Default service account to use for workload identity when not specified in resources |
| `--enable-leader-election` | `true` | Enable leader election for controller manager |
| `--health-addr` | `:8081` | The address the health endpoint binds to |
| `--interval-jitter-percentage` | `5` | Percentage of jitter to apply to interval durations |
| `--leader-election-lease-duration` | `35s` | Interval at which non-leader candidates will wait to force acquire leadership |
| `--leader-election-release-on-cancel` | `true` | Defines if the leader should step down voluntarily on controller manager shutdown |
| `--leader-election-renew-deadline` | `30s` | Duration that the leading controller manager will retry refreshing leadership before giving up |
| `--leader-election-retry-period` | `5s` | Duration the LeaderElector clients should wait between tries of actions |
| `--log-encoding` | `json` | Log encoding format. Can be 'json' or 'console' |
| `--log-level` | `info` | Log verbosity level. Can be one of 'trace', 'debug', 'info', 'error' |
| `--max-retry-delay` | `15m` | The maximum amount of time for which an object being reconciled will have to wait before a retry |
| `--metrics-addr` | `:8080` | The address the metric endpoint binds to |
| `--min-retry-delay` | `750ms` | The minimum amount of time for which an object being reconciled will have to wait before a retry |
| `--override-manager` | | Field manager disallowed to perform changes on managed resources (repeatable) |
| `--reporting-interval` | `5m` | The interval at which the report is computed |
| `--requeue-dependency` | `5s` | The interval at which failing dependencies are reevaluated |
| `--storage-path` | `/data` | The local storage path |
| `--token-cache-max-duration` | `1h` | The maximum duration a token is cached |
| `--token-cache-max-size` | `100` | The maximum size of the cache in number of tokens |
| `--watch-configs-label-selector` | `reconcile.fluxcd.io/watch=Enabled` | Watch for ConfigMaps and Secrets with matching labels |
| `--web-config` | | The path to the configuration file for the web server |
| `--web-config-secret-name` | | The name of the Kubernetes Secret containing the web server configuration |
| `--web-server-only` | `false` | Run only the web server without starting the controllers |
| `--web-server-port` | `9080` | The port for the web server to listen on (0 to disable) |

The flags can be passed to the operator container using the
[Helm chart](https://github.com/controlplaneio-fluxcd/charts/tree/main/charts/flux-operator) values `extraArgs`.

## Environment Variables

The following environment variables are available:

| Env Var | Description |
|---------|-------------|
| `DEFAULT_SERVICE_ACCOUNT` | Default service account used for impersonation |
| `DEFAULT_WORKLOAD_IDENTITY_SERVICE_ACCOUNT` | Default service account for workload identity |
| `OVERRIDE_MANAGERS` | Comma-separated field managers disallowed to perform changes |
| `REPORTING_INTERVAL` | The interval at which the report is computed |
| `WEB_SERVER_PORT` | The port for the web server |
| `WEB_CONFIG_SECRET_NAME` | Secret name for web server configuration |

The environment variables are intended for Operator Lifecycle Manager (OLM) installations
and can be set in the `Subscription` manifest using the `config.env` field.