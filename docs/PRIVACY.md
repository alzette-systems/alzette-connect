# Alzette Connect privacy

Effective: 3 September 2026

This notice covers the open-source Alzette Connect desktop application. The
Alzette website, hosted identity service, inference platform, and customer
administration services are separate systems and are not part of this
repository or its Apache-2.0 license.

## Information the desktop application handles

Alzette Connect does not include analytics, advertising, or crash-reporting
SDKs. It handles the following information to provide features that the user
explicitly requests:

- When the user chooses to sign in, Connect retrieves configuration from
  `app.alzette.systems`, opens the configured identity provider in the system
  browser, exchanges the returned OAuth authorization code, and retrieves the
  workspaces and model catalogue available to that account.
- Connect stores the OAuth refresh credential in the operating system's
  protected credential store. Access credentials, private loopback
  capabilities, and model-session credentials are kept in memory or in the
  protected credential store, not in plaintext configuration files.
- When the user launches a supported AI client, Connect runs a private proxy
  on the local computer. Requests made through that proxy—including prompts,
  conversation content, selected models, and response metadata—are forwarded
  to the Alzette inference gateway so the requested inference can be
  performed.
- Connect reads the installed versions and relevant local configuration of
  supported AI clients. When authorized by the user, it makes bounded,
  reversible configuration changes and launches the selected client.
- When the user asks Connect to check for or download an update, it contacts
  the GitHub API and GitHub release-download infrastructure for the public
  `alzette-systems/alzette-connect` repository. GitHub receives ordinary web
  request information such as the user's IP address and a user-agent containing
  the installed Connect version.

The application listens only on loopback addresses for its OAuth callback and
local proxy. Those local connections are not transfers to another organization.

## Retention and control

Signing out removes the locally stored Alzette refresh credential and asks the
service to revoke the active credential. Uninstalling the application does not
silently delete configuration owned by other AI clients. Server-side processing
and retention are governed by the agreement between the user's organization and
the operator of the Alzette service.

The default service and update endpoints are visible in this repository. A
development build can be configured to use a different Alzette control endpoint;
the operator of that endpoint is then responsible for its own data practices.

## Contact

Questions about this notice can be sent to `engineering@alzette.systems`.
