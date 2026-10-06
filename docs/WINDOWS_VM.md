# Local Windows test VM

The Debian development host has a dedicated VM named `alzette-windows-qa` on
`qemu:///system`. It does not autostart. Its disk and official installation
media live under `/var/lib/libvirt/images/alzette-windows/`; the installation
and private setup seed media have been ejected after installation.

```sh
virsh -c qemu:///system start alzette-windows-qa
virt-viewer --connect qemu:///system alzette-windows-qa
```

Use `virsh -c qemu:///system domstate alzette-windows-qa` first; `start` is only
needed when the guest is off. The VNC console is bound to `127.0.0.1:5900`.
If accessing the host remotely, forward that localhost port through SSH and
connect a VNC viewer through the tunnel. No public VNC listener is required.

The guest is `alzette-win-qa`, with QA account `alzette-test` and address
`192.168.139.2` on the dedicated NAT network `alzette-windows-net`. Its local OS
password and SSH key are in the private host directory
`/root/.local/share/alzette-windows-qa/`. Do not include that directory or the
setup seed ISO in test reports, releases, or source control.

```sh
ssh -i /root/.local/share/alzette-windows-qa/ssh-key alzette-test@192.168.139.2
```

SSH works for inspection and native command tests. Launch desktop applications
from the logged-in console; an SSH service session does not provide an
interactive Windows desktop. Application installers, the internal Connect
build and test reports are in `C:\AlzetteQA`. The dummy workspace is
`C:\AlzetteQA\workspace`.

The installed applications and remaining checks are recorded in
[DESKTOP_PARITY.md](DESKTOP_PARITY.md). The ChatGPT native fixture is documented
in [WINDOWS_CHATGPT_PARITY.md](WINDOWS_CHATGPT_PARITY.md); the Claude fixture is
in [WINDOWS_CLAUDE.md](WINDOWS_CLAUDE.md). Both fixtures require no tenant
credentials and do not call paid models. Real employee testing must use the
normal browser Casdoor/OIDC flow. Disposable Windows QA identity credentials
are kept outside the repository in the private host directory, never in evidence.

Windows work currently prioritizes ChatGPT and Claude Desktop. Pi, Jan and Goose
remain installed for optional later work; macOS acceptance is independent.

When finished, shut the guest down normally:

```sh
virsh -c qemu:///system shutdown alzette-windows-qa
```

Keep the VM and disk for repeat acceptance work. Do not remove or modify the
unrelated host VMs or production service networks.
