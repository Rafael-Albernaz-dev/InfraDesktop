# InfraDesktop

Local desktop workspace for ISP technicians, independent from Atlas. Ubuntu 24.04 amd64, Go/Wails and React/TypeScript. Version 0.6.0 provides a real local account, offline recovery with copy/text export, remembered/revocable access, isolated company databases, per-company branding and themes through an independent daemon. It adds explicit company inventory, reusable encrypted read credentials, explicit first host-key acceptance, integrated equipment summaries and finite on-demand SSH reads of identity, resources and interfaces. Controlled v6/v7 fixtures exercise the adapter; real firmware compatibility and operational provisioning require separate evidence. Other operational panels remain unavailable without observations.

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

`make test-integration` uses private temporary state and owned daemon processes. `make test-desktop` drives the optimized real Wails/GTK/WebKit binary on its own X11/Xvfb display, with real SQLite and an isolated technical account. The presentation fixture (`make test-visual`) captures44 historical/variant states at eight viewports:1440×900,900×1440,768×1024,1366×768,1280×720,1100×700,1440×860 and720×600. `INFRADESKTOP_VISUAL_VIEWPORTS=1440x860,720x600` selects an explicitly incomplete matrix. Whole-client comparisons use `INFRADESKTOP_VISUAL_REFERENCES`; the resolved origin reaches the inspector through the capture manifest. Missing references report fidelity NOT RUN; capture PASS is separate from visual review. Source/bundle/reference/capture hashes invalidate stale comparisons and reviews. Historical Equipment presentation remains in this broad fixture; current Equipment has its own smoke below. Neither test constitutes human acceptance, physical equipment support or proof of Linux boot/logout.

The isolated L04/L05 journey uses `make build test-part1-desktop`: the normal UI binary, a separate uninstalled daemon with an explicit test key, and an owned loopback SSH peer. It covers account/company selection, enrollment/acceptance, full reading, active/inactive edits without implicit SSH, dashboard and reopen at720×600 in both themes. State/key stay together in its reported synthetic root; cleanup stops only owned children. `make test-part1-production` builds and proves the normal daemon ignores the DEV key; `make test-vault-helper` checks provisioning logic without host operations; `make test-vault-read-race` schedules a permission race through the real provider with Go race detection. CI includes these gates.

`make test-equipment-smoke` exercises current Equipment components in12 states at1440×860 and720×600, both themes: empty/list/no-results/inactive-only/vault and forms/trust/error-retry/result. This public fixture requires no private references and explicitly reports fidelity NOT RUN. `make test-part1-visual` uses the same harness plus approved reference context from `INFRADESKTOP_VISUAL_REFERENCES` (default `docs/reference/infradesktop-ui-0.2.1`); missing references fail explicitly. Timestamped output is under `build/equipment-visual/`; `INFRADESKTOP_EVIDENCE_DIR` selects another output. Compare with `python3 scripts/compare-visual.py --captures <output>`; acceptance requires recorded pair/overlay inspection. Evidence wrappers also support `INFRADESKTOP_EVIDENCE_DIR` and preserve earlier attempts. None of these gates connects to an ISP or provisions the installed service.

## Package and installation

`make package` creates and inspects `build/infradesktop_0.6.0_amd64.deb`: two binaries, launcher, icons/licenses and a system service template. Dependencies come from the binaries plus explicit systemd and python3 for the provisioning helper; the unit is checked using systemd-analyze. No maintainer script enables a service. Installation requires administrator authorization in the local terminal:

```bash
sudo dpkg -i ./build/infradesktop_0.6.0_amd64.deb
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

A remembered grant is random, distinct from an interactive session; SQLite stores only its digest. Remote passwords use AES-256-GCM envelopes and company data keys wrapped under an independent installation master. The production provider reads only a systemd runtime credential; absence/corruption leaves inventory usable and the vault unavailable. Explicit administrative provisioning uses `provision-vault` with a host-only encrypted credential and prepares a drop-in; it never reloads or restarts the service. Account recovery preserves the vault but cannot replace a lost host master. Filesystem/envelope controls do not defend against root or malicious same-UID processes. Existing unsafe/stale endpoints fail closed. After an unclean DEV crash, inspect exact ownership before removing any stale resource; do not delete a human database to recover.

Remote CI, clean Ubuntu, Linux boot/logout, native Wayland and4GB hardware acceptance require their own evidence. No product license has been selected; bundled third-party notices are under `packaging/linux/licenses` and `frontend/public/fonts`.

PPPoE A adds deliberate active-session reads, duplicates/filter/order/page/detail, one current reading and protected references per concentrator, conservative local comparison and a sanitized JSON export through the native picker. Incomplete/limited work preserves the previous good reading. IPC revision8 is required for the UI/daemon pair; frames remain64KiB and SSH jobs512KiB. The existing shared2048 durable-operation policy is preserved. VLAN/RADIUS/ping are reserved for a later delivery.

The isolated A journey uses the binaries from `make package part1-harness`, then `dbus-run-session -- xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/test-l06-desktop.py`. It creates only owned synthetic account/company/SQLite/vault/SSH roots, tests both themes at720×600 including physical controls, native picker and real process reopen, and closes its children. `node scripts/test-l06-states.mjs` exercises the current A component in isolated browser states at720/1440 in both themes without private references. The `l06measure` Go build tag enables the real IPC/SQLite/SSH workload meter in `internal/adapters/ipc/pppoe_measure_test.go`; results include rejected loads and do not claim50k-session or4GB support. None of these commands installs, publishes or homologates firmware.
