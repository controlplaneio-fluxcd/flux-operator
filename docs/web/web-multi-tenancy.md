---
title: Flux Web UI Multi-Tenancy
description: Flux Status Web UI multi-tenancy guide.
---

# Flux Web UI Multi-Tenancy

The Flux Web UI supports multi-tenancy by allowing users to view and manage Flux resources
across multiple namespaces in a single interface.
This is achieved through the use of Kubernetes Role-Based Access Control (RBAC) policies
that grant permissions to specific namespaces.

## Namespace Access

The Web UI follows the Flux multi-tenancy model where users can have access to resources in multiple namespaces.
The cluster admins are responsible for granting users access to specific namespaces by creating `RoleBindings`
that bind users or groups to [predefined roles](web-user-management.md#predefined-roles) such
as `flux-web-user` or `flux-web-admin`.

The Web UI detects which namespaces a user has access to based on the `get` permission for the `ResourceSet` kind.
By default, the Flux Operator Helm chart creates a role that grants access to `ResourceSet`
by aggregating to `view` and `edit` roles.

The Web UI maintains a list of namespaces that a logged-in user has access to and filters the resources displayed
in the UI accordingly. The list of namespaces is refreshed periodically (every `20s`) to ensure it reflects
any changes in permissions.

## Flux Resources Access

When a user has access to multiple namespaces, the Web UI allows them to view and manage
Flux resources across those namespaces.

### Search Permissions

If a user has `get` permission for `ResourceSet` in a namespace,
the Web UI will include that namespace in the dropdowns for filtering and searching resources.

Note that the Web UI only checks for `get` permission on `ResourceSet` to determine namespace read access,
if a user has `get` permission for `ResourceSet` but not for other Flux resources (e.g., `HelmRelease`),
they will be able to see the HelmRelease resources in the search results, but will receive a "403 Forbidden"
error when trying to view the details of a specific HelmRelease. 

### Resource Details Permissions

If a user has `get` permission for a specific Flux resource in a namespace, they will be able to view the details
of that resource in the Web UI, including its YAML spec, status and the list of managed objects.

For the details view to work properly, users also need `get` permission
for Kubernetes `Event`, `Deployment`, `DaemonSet`, `StatefulSet`, `CronJob`, `Job`, `Pod`
resources. This is covered by the `flux-web-user` role, but if you have custom roles,
make sure to include these permissions.

!!! warning "HelmRelease Inventory Permissions"

    To view the inventory of a `HelmRelease` resource, users also need `get` permission for Kubernetes `Secret`
    resources in the same namespace. This is because Helm stores the release information in a Secret,
    and the Web UI needs to access it to display the inventory.

    Starting with Flux v2.8, the HelmRelease status embeds the inventory information, so users with `get` permission
    for `HelmRelease` will be able to see the inventory without needing access to secrets.

### Actions Permissions

Actions are only available when [authentication](web-user-management.md) is configured.
Each action requires specific RBAC permissions, granted through the `flux-web-admin`
role or custom roles with the necessary verbs.

Please the [Actions documentation](web-actions.md) for details on the required permissions for each action.
