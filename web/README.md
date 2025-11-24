# Flux Status Page

**Mission control dashboard for Kubernetes app delivery powered by Flux CD**

The **Flux Status Page** is a lightweight, mobile-friendly web interface providing real-time
visibility into your GitOps pipelines. Embedded directly within the **Flux Operator**,
it requires no additional installation steps.

Designed for DevOps engineers and platform teams, the Status Page offers direct insight
into your Kubernetes clusters. It allows you to track app deployments, monitor
controller readiness, and troubleshoot issues instantly, without needing to access the CLI.

Built with security in mind, the interface is strictly read-only, ensuring it never
interferes with Flux controllers or compromises cluster security.
Together with the **Flux MCP Server**, it provides a comprehensive solution for
on-call monitoring and Agentic AI incident response in production environments.

## Features

- **Operational Overview:** View the real-time status and readiness of all Flux controllers.
- **Monitor Reconciliation:** Monitor the sync state of your cluster and infrastructure deployments.
- **Pinpoint Issues:** Quickly identify and troubleshoot failures within your app delivery pipelines.
- **Navigate Efficiently:** Use advanced search and filtering to find specific resources instantly.
- **Deep Dive:** Access dedicated dashboards for ResourceSets, HelmReleases, Kustomizations and Flux sources.
- **Mobile-Optimized:** Stay informed with a fully responsive interface designed for on-the-go checks.
- **Adaptive Theming:** Toggle between dark and light modes to suit your environment and preference.

## Accessing the Status Page

The Flux Status Page is exposed on port `9080` by the `flux-operator` Kubernetes service.

To access the Status Page, you can port-forward the service to your local machine:

```bash
kubectl -n flux-system port-forward svc/flux-operator 9080:9080
```

To expose the Status Page externally, you can create an Ingress or Gateway HTTPRoute resource
pointing to the `flux-operator` service on port `9080`.

> [!IMPORTANT]
> Ensure you secure access to the Flux Status Page appropriately!
> While the UI is read-only and doesn't show sensitive data from Kubernetes secrets,
> it does expose details about your cluster's infrastructure and app deployments.

## Contributing

We welcome contributions to the Flux Status Page project via GitHub pull requests.
Please see the [CONTRIBUTING](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/CONTRIBUTING.md)
guide for details on how to set up your development environment and start contributing to the project.

## License

The Flux Status Page is open-source and part of the [Flux Operator](https://github.com/controlplaneio-fluxcd/flux-operator)
project licensed under the [AGPL-3.0 license](https://github.com/controlplaneio-fluxcd/flux-operator/blob/main/LICENSE).
