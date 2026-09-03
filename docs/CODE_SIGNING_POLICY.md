# Code signing policy

Alzette Connect's source and release automation are maintained in the public
[`alzette-systems/alzette-connect`](https://github.com/alzette-systems/alzette-connect)
repository. The desktop client is licensed under Apache-2.0. The hosted Alzette
website and inference platform are separate products and are not included in
the signed artifacts.

Free code signing provided by SignPath.io, certificate by SignPath Foundation.

## Signed artifacts

Release signing covers the Windows application executable and the final Windows
installer. Signing requests must originate from the repository's GitHub Actions
release workflow. SignPath origin verification binds each request to the public
repository, workflow, commit, and release reference. Released artifacts also
carry GitHub build-provenance attestations and SHA-256 digests.

No maintainer may submit an unrelated binary, a locally built binary, a binary
from another repository, or proprietary Alzette server code for signing under
this project.

## Team roles

The project is currently maintained by one person, who holds the roles required
for a single-maintainer open-source project:

- Committer and reviewer: [@ticruz38](https://github.com/ticruz38)
- Signing approver: [@ticruz38](https://github.com/ticruz38)

Contributions from people without direct commit access require review before
merge. Release signing is limited to reviewed source on the protected release
path. Maintainers must enable multifactor authentication for both GitHub and
SignPath. If the maintainer team changes, this policy and the corresponding
SignPath roles must be updated before the new member can approve a release.

## Privacy

The desktop application's network behavior and data handling are documented in
the [Alzette Connect privacy notice](PRIVACY.md).
