# Fjord (`fjordd`) — Technical Design Document (v1.0)

## 1. Introduction & Objectives
Fjord is a lightweight, FreeBSD-native OCI Compose Manager and Container App Store. It bridges the gap between standard OCI container workflows (Podman/Compose) and FreeBSD’s native host capabilities (ZFS, devfs, Jails) without forcing users into proprietary formats.

### 1.1 Goals
- **1-Click App Store Experience:** Provide an intuitive catalog with sane defaults for rapid deployment.
- **Compose-First Foundation:** Maintain Dockge parity. The `compose.yaml` and `.env` files on disk are the ultimate source of truth, remaining editable via an in-browser tabbed text editor.
- **FreeBSD Native:** Automate ZFS dataset creation, permissions, and devfs rules prior to container launch.
- **Zero Dependencies:** A single, statically compiled Go binary (`CGO_ENABLED=0`) with an embedded Svelte frontend.

### 1.2 Non-Goals
- **Complex Network Orchestration:** Fjord will not contain custom Go code to manage VNETs or pf NAT rules. We will rely on Podman/Netavark for network translation.
- **Multi-Node Swarm:** MVP is strictly single-node host management.

---

## 2. System Architecture
Fjord consists of a pure Go backend and an embedded Svelte frontend, distributed as a single static binary.

### 2.1 The Execution Engine
To satisfy the strict `CGO_ENABLED=0` constraint, Fjord acts as an orchestration wrapper rather than a low-level container runtime:
- **State & Telemetry:** Fjord talks to the podman UNIX socket (`/var/run/podman/podman.sock`) through a small hand-rolled HTTP client (`pkg/engine/podman/socket.go`), not the podman Go bindings — the bindings drag in cgo and most of podman, which breaks the `CGO_ENABLED=0` single-binary goal. It reads structured container states, ports, volumes, networks, and drives the exec bridge; live CPU/RAM metrics are not implemented yet.
- **Lifecycle Execution:** For `up`, `down`, `update` and `restart`, Fjord uses Go's `os/exec` to run `podman-compose` (podman) or `appjail-director` (AppJail — the compose-shaped tool on that side; `appjail oci run` is to it what `podman run` is to `podman-compose`). `stdout` and `stderr` are streamed to the UI for a live terminal experience. An AppJail stack is materialized from the catalog's dbuild bundle: `appjail-director.yml`, `Makejail` and a jail template, with the wizard's values in `.env`.
- **Engines:** every runtime implements `engine.Backend` (`pkg/engine`) and registers a `Descriptor` (name, availability probe, install package). podman and AppJail are built in; a stack is bound to one engine at install (`state.json`), and everything above the interface — handlers, UI, catalog — stays runtime-agnostic. A stack whose engine is unavailable reports that; nothing is silently rerouted.
- **Stack identity:** a stack's id is the slug of its name (`radarr`, then `radarr-2`), unique among existing stacks — compose/Portainer style. It is the directory, the compose project and therefore the prefix of the stack's containers, network and named volumes. Deleting a stack frees the name; a reinstall under the same name picks up the named volumes Delete kept, exactly like `compose down` then `up`. The display name is a free label on top of the id.

### 2.2 Resources: neutral concepts, engine translation

The compose file is Fjord's **single source of truth** for a stack — including its
resources (storage, networking, CPU/RAM limits, and engine-specific options).
Fjord never keeps resource state in a separate sidecar; the **Resources** tab is a
friendly *editor over the compose*, and each `engine.Backend` **translates the
neutral compose keys into its own runtime mechanism at `Up` time**. This is the
same "generic core, translation in the engine" contract as `engine.Descriptor`:
handlers and UI speak compose; only backends know podman flags or jail params.

| Concern | Neutral compose key | podman applies | appjail applies |
|---|---|---|---|
| Storage | `volumes:` (bind or named) | native mounts | fstab pseudofs |
| Networking | `networks:` / `ports:` | native / macvlan | virtualnet + expose |
| CPU | `cpus:` / `cpuset:` | `--cpus` / `--cpuset-cpus` | `rctl pcpu` / cpuset |
| Memory | `mem_limit:` | `--memory` | `rctl memoryuse:deny` |
| Processes | `pids_limit:` | `--pids-limit` | `rctl maxproc` |
| Jail options (mlock, sysvipc, vnet, …) | `org.freebsd.jail.*` annotations | ignored | jail params (`-o start=`) |

Consequences:

- **Every Resources section is the same pattern:** read structured keys from the
  compose → render friendly editors → write them back → engine translates at run
  time. Storage is the first slice; Limits and Jail-options reuse it wholesale, so
  there is no per-feature storage subsystem.
- **Stacks stay portable.** A stack authored on podman still expresses its limits
  in standard compose keys; move it to appjail and the appjail backend interprets
  the same keys. Nothing resource-related lives outside the compose.
- **`rctl` gap:** compose has no native key for FreeBSD `rctl`, so the appjail
  backend derives its rules from the standard keys (e.g. `mem_limit` →
  `rctl jail:<name>:memoryuse:deny`). The compose remains canonical; appjail is
  purely an interpreter.
- **Jail options already flow** through `org.freebsd.jail.*` annotations (that is
  how `allow.mlock`/`allow.sysvipc` work today); the Advanced section just surfaces
  the annotations that already reach the appjail backend.

Planned Resources tab layout: **Storage** (mounts table) · **Networking**
(host-ports / own-IP) · **Limits** (CPU · RAM · PIDs) · **Advanced**
(engine-specific, e.g. jail params — collapsed).

---

## 3. Storage
Three kinds of storage, kept apart on purpose:

1. **fjord's own files** — `/var/db/fjord`: `stacks/<id>/{compose.yaml,.env,state.json}`, `settings.json`, the catalog cache. Internal; never edited by users.
2. **App data** — where each app's own folders go, `<location>/<app slug>/<name>` (default `<fjord root>/containers`; the user usually points it at `/containers`). An ordered list of locations, the first being the default; with several, the install wizard offers a pick. Folders are created with `mkdir -p` + `chown 1000:1000`; an existing folder is adopted as-is, so bringing in existing data = install the app under that name.
3. **Folder sets** — named sets of host folders the user already has (movies, downloads…). Any host-path field in the wizard is a folder list; a set drops its folders in, several folders mount as uniquely-named sub-folders of the app's mount point. Entries may be `nfs://server/export` or `smb://user@server/share`, which become named volumes created on first use. Preset sets with automatic pick-up are the **Homelab** plugin.

`{{stack}}` (the app's folder name) and `{{appdata}}` (the chosen App data location) may appear in any host path and are resolved at install.

### 3.1 ZFS (planned)
Dataset-per-app under a delegated `zroot/fjord`, snapshot-before-update and `zfs rollback` restore are the M4 plan. Today no `zfs` call exists; the layout above is what a later dataset backing would preserve.

---

## 4. Networking Strategy (The Netavark Drop-in)
Podman 5.x is moving toward Netavark natively on FreeBSD, which will automate epair interfaces and pf NAT rules natively. Fjord's architecture anticipates this shift:
- **Phase 1 (MVP):** Fjord delegates all networking to Podman. We rely on standard `network_mode: host` or the basic port mappings supported by current ocijail implementations.
- **Phase 2 (Netavark Arrival):** Fjord requires zero code changes. `podman-compose` will instruct Podman, and Podman will use Netavark to automatically handle routing.
- **Host Pre-Flight:** Fjord's only network responsibility is a pre-flight check validating that Packet Filter (`pf`) is enabled on the host, warning the user if it is not.

---

## 5. Security & Authentication
Fjord runs as root (required for provisioning app folders, jail parameters, and the podman socket).

**Status (tech preview): there is no authentication.** Anyone who can reach the port can run containers as root. Mitigations available today: keep it on a trusted LAN, or bind to loopback (`FJORD_LISTEN=127.0.0.1:3567`) and reach it over SSH.

Planned before any non-LAN exposure (M3):
- A login screen protecting the whole application; first boot creates the admin account.
- Credentials hashed (bcrypt) in `/var/db/fjord/fjord-auth.json`; login sets an HTTP-only session cookie; every REST endpoint and WebSocket upgrade validates it.
- Same-origin enforcement (Origin / Sec-Fetch-Site) and a required JSON Content-Type on mutating requests, so a visited web page cannot drive the API.
- SMB passwords already stay out of the API surface: they are stored root-only on the host (`/etc/nsmb.conf` managed block on FreeBSD, credentials file on Linux) and never appear in composes, settings or podman metadata.

---

## 5.5 Bootstrap Configuration
Everything a user can set *through* the Web UI (catalogs, App data locations, folder sets, engines, plugins) lives in `/var/db/fjord/settings.json`, written atomically. A handful of settings must be known *before* the UI can come up — there's no way to fix the listen port through a UI you can't reach because the port is wrong. Those are **environment variables** (`FJORD_LISTEN`, `FJORD_STACKS_DIR`, `FJORD_STORAGE_BASE`, `FJORD_ENGINE`, `FJORD_PODMAN_SOCKET`, `FJORD_HOST_ADDR`), set via `fjordd_env` in `rc.conf` on FreeBSD. That is deliberately the whole bootstrap story: plain, recoverable from a headless host, and Ansible-friendly. A `/usr/local/etc/fjord.yaml` file is not implemented; if TLS or more boot-time knobs arrive, it may be.

---

## 6. UI & UX Flow (Svelte + Vite)
Fjord embeds a Svelte Single Page Application (SPA) into the Go binary using the `embed` package. The UI is designed to balance a consumer-friendly "App Store" with "Pro-level Control".

### 6.1 The Dashboard (Landing Page)
- Displays a grid of currently installed Stacks.
- Live status badges (Running, Stopped, Degraded) updated via Podman socket polling.
- Clicking a stack opens the Editor View: a tabbed CodeMirror editor for the stack's files (`compose.yaml` / `.env`, or `appjail-director.yml` / `Makejail` / `.env` on AppJail) plus a Resources form, with a drawer for engine output, logs and a shell.

### 6.2 The App Store Catalog
- A dedicated tab pulling a JSON manifest from a static catalog (e.g., GitHub Pages: `daemonless/fjord-catalog`).
- **1-Click Install Flow:**
  1. User clicks "Install Nextcloud".
  2. A modal appears with sane defaults pre-filled (e.g., Port 8080, standard volume mounts).
  3. User clicks "Deploy".
  4. **Execution:** Fjord creates the app's folders under the App data location (`<appdata>/nextcloud/config`, chowned to PUID/PGID), writes `stacks/nextcloud/compose.yaml` + `.env` (or the director bundle), runs pre-flight (ports, folders), triggers the engine's `up`, and streams the output to the UI.
