# InfraDesktop

Local desktop workspace for ISP technicians, independent from Atlas. Ubuntu 24.04 amd64, Go/Wails and React/TypeScript. Version 0.4.0 provides a real local account, offline recovery with copy/text export, remembered/revocable access, isolated company databases, per-company branding and themes through an independent daemon. Device queries, remote credentials/vault and AI are not implemented; operational panels show unavailable readings rather than demo data.

## Build and checks

Pinned tools: Go 1.27.3, Wails CLI v2.17.0, Staticcheck v0.8.2, Node 24.16.0/npm 11.17.0. Required headers: GTK3/WebKitGTK4.1, libwebp, GCC and pkg-config. Packaging uses Python3, dpkg-dev and systemd-analyze. Visual tests use the installed `/usr/bin/google-chrome` with Playwright1.64.1; native tests use Xvfb/xauth, Python GI/AT-SPI/GdkX11 and libXtst. Icon regeneration/check uses Pillow10.2.0. These tools are prepared explicitly; gates never install tools or browsers.

```bash
export PATH="$PWD/.tools/go1.27.3/go/bin:$PWD/.tools/bin:$PATH"
export GOTOOLCHAIN=local
go mod download
npm --prefix frontend ci
make tools-check check test-integration build package test-visual test-desktop
```

`make bindings` generates the narrow Wails adapter; `make build` builds frontend assets before Go embed and creates `build/bin/infradesktop` and `build/bin/infradesktopd`. The `webkit2_41` tag is used on Ubuntu24.04. `npm ci` creates an ignored Go module boundary in node_modules, keeping npm-shipped Go samples outside product checks. Public build/runtime never depends on the private docs or agent workspace.

## Development

```bash
make run
```

This supervises only its own DEV daemon/window, without sudo or demo seeds. Durable state is `.local/bf01/state`, the Go client's remembered credential is `.local/bf01/client`, and each invocation gets a private directory under `.runtime/bf01`. Readiness requires the child UI to render its service status through AT-SPI; launch failures and unexpected exits propagate. Closing the DEV window stops these DEV children; data persists. The installed service has an independent lifecycle. Never reuse a test state as a human account.

Create the single account on first use. Passwords accept15–128 Unicode characters, including spaces. Save the one-time recovery code offline; acknowledge it, then log in. Create/select a company and open the workspace. Remembered access resumes after reopening and daemon restart; app logout revokes it durably. Recovery requires the offline code, rotates it, replaces the password and revokes previous sessions/grants. There is no default password or recovery by email. Losing both password and code does not promise account recovery.

`make test-integration` uses private temporary state and owned daemon processes. `make test-desktop` drives the optimized real Wails/GTK/WebKit binary on its own X11/Xvfb display, with real SQLite and an isolated technical account; it verifies registration, companies, themes, reopen/restart, logout and recovery. The separate presentation fixture entry (`make test-visual`) exercises the shared React components without replacing production access bindings or seeding product state. It captures 20 states at 1440×860, 1366×768 and compact 1088×768, and generates whole-client comparisons when the explicit private reference is available (`INFRADESKTOP_VISUAL_REFERENCES`). Missing references report fidelity NOT RUN; capture PASS is separate from visual review. Source/bundle/reference/capture hashes invalidate stale comparisons and reviews. Neither test constitutes human acceptance, physical equipment support or proof of Linux boot/logout.

## Package and installation

`make package` creates and inspects `build/infradesktop_0.4.0_amd64.deb`: two binaries, launcher, icons/licenses and a system service template. Dependencies are derived from the binaries; the unit is checked using systemd-analyze. No maintainer script enables a service. Installation requires administrator authorization in the local terminal:

```bash
sudo dpkg -i ./build/infradesktop_0.4.0_amd64.deb
sudo /usr/lib/infradesktop/provision-technician "$(id -un)"
gtk-launch infradesktop
```

The provisioner validates an existing canonical non-root Unix account. For technician `toru`, the service is `infradesktopd@toru.service`, runtime `/run/infradesktop-toru` (0700), durable state `/var/lib/infradesktop-toru` (0700), SQLite0600. The client credential defaults to `~/.local/state/infradesktop/remembered` (0600; directory0700). The daemon rejects root; ProtectHome/ProtectSystem remain enabled and systemd grants writes only to its private runtime/state. The installed launcher needs neither environment overrides nor a second terminal.

Restart only the selected InfraDesktop instance. Package preparation is not proof of installation; check actual package/service status on the target. Linux reboot/logout require a separate human window. Remove only this product when rolling back; do not delete account data automatically:

```bash
sudo systemctl disable --now "infradesktopd@$(id -un).service"
sudo dpkg --remove infradesktop
```

## Security and limits

IPC uses an owner-only Unix endpoint, peer UID validation in both directions, strict versioned framing and a separate channel capability. Account authorization stays in the daemon. Argon2id uses a random16-byte salt,32-byte hash and persisted/validated m19456KiB/t2/p1 parameters; attempts are throttled durably and KDF concurrency is bounded. Passwords, grants and capabilities are never logged or stored by browser/localStorage. Only the legitimate password/recovery input and one-time code display transiently enter JS.

A remembered grant is random, distinct from an interactive session; SQLite stores only its digest. Filesystem/UID controls do not defend against root or malicious same-UID processes and are not an encrypted vault. Existing unsafe/stale endpoints fail closed. After an unclean DEV crash, inspect exact ownership before removing any stale resource; do not delete a human database to recover.

Remote CI, clean Ubuntu, Linux boot/logout, native Wayland and4GB hardware acceptance require their own evidence. No product license has been selected; bundled third-party notices are under `packaging/linux/licenses` and `frontend/public/fonts`.
