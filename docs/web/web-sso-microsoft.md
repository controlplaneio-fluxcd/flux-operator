---
title: Flux Web UI SSO with Microsoft Entra
description: Flux Status Web UI SSO guide using Microsoft Entra as the identity provider.
---

# Flux Web UI with Microsoft Entra SSO

There are several ways to configure Single Sign-On (SSO) for the Flux Web UI using
Microsoft Entra (formerly Azure Active Directory) as the identity provider. This guide
covers two common approaches: direct integration with Microsoft Entra OIDC, and using
Dex as an intermediate OIDC provider.

## Direct Integration with Microsoft Entra OIDC

When deploying Flux Operator through the Helm chart, you can configure the Flux Web UI
to use Microsoft Entra OIDC with a configuration similar to the following:

```yaml
config:
  baseURL: https://flux-status.example.com
  authentication:
    type: OAuth2
    oauth2:
      provider: OIDC
      clientID: 2d01bd48-7914-4b50-9667-068be6afd2f2           # App registration's "Application (client) ID" value.
      clientSecret: "O.y8Q~MGIm9B.ahUlOx376EP7l5mu9xgIet6hdBD" # App registration's "Client secret" value.
      issuerURL: https://login.microsoftonline.com/4bd94393-a3a0-ab26-4c05-bfc69377f6c0/v2.0 # URL containing tenant ID.
      scopes: [openid, profile, email, offline_access]         # Scopes supported by Microsoft Entra OIDC.
```

In order to receive groups from Microsoft Entra, you need to configure the
App registration to include the `groups` claim in the ID token by following
[these](https://learn.microsoft.com/en-us/entra/identity-platform/optional-claims#configure-groups-optional-claims)
docs.

!!! note "Limitations"

    This approach has a limitation for Azure free-tier plans, where the UUIDs
    of the groups are returned instead of their names. For receiving group
    names, you need to choose the "Groups assigned to the application" type
    and have a paid plan. Alternatively, you can use Dex as an intermediate
    OIDC provider to get group names, as described in the next section.

## Using Dex as an intermediate OIDC provider

To receive groups from Microsoft Entra and get more advanced features, you can
use Dex as an intermediate OIDC provider between the Flux Web UI and Microsoft
Entra. For this, please refer to the guide [Flux Web UI SSO with Dex](./web-sso-dex.md)
and the docs for the [Dex Microsoft connector](https://dexidp.io/docs/connectors/microsoft/).

### Restricting the groups added by Dex to the ID token

The Microsoft Entra connector in Dex supports filtering the groups
added to the ID token using various configuration options. Example:

```yaml
connectors:
  - type: microsoft
    id: microsoft
    name: Microsoft
    config:
      clientID: 2d01bd48-7914-4b50-9667-068be6afd2f2
      clientSecret: "O.y8Q~MGIm9B.ahUlOx376EP7l5mu9xgIet6hdBD"
      redirectURI: https://dex.example.com/callback
      onlySecurityGroups: true   # only groups created with the "Security" type
      useGroupsAsWhitelist: true # only include groups listed in "groups" below
      groups: # user needs at least one of these groups to be authenticated
        - dev-team
        - platform-team
```

This can be useful to shorten the list of groups added to the ID token,
and therefore reducing the overall size of the token. The Web UI supports
a maximum of 35 KiB for the entire HTTP cookie used to store the ID token.

## Further Reading

- [Flux Web UI SSO with Dex](./web-sso-dex.md)
- [Flux Web UI Ingress Configuration](./web-ingress.md)
- [Dex Microsoft connector](https://dexidp.io/docs/connectors/microsoft/)
- [Add the groups claim for direct integration](https://learn.microsoft.com/en-us/entra/identity-platform/optional-claims#configure-groups-optional-claims)
