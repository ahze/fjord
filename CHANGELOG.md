# Changelog

## 0.2.1 — 2026-09-13

- **Fix:** the pf readiness fixes appended the anchors to the end of
  `/etc/pf.conf`. On a host with `pass`/`block` rules that put translation
  after filtering, pf rejected the whole file and the rc script left pf
  running with nothing loaded — all NAT gone at once. The snippet now
  inserts before the first filter rule and validates with `pfctl -nf`
  before loading. If you pasted the 0.2.0 fix: move the anchor lines above
  your filter rules and `pfctl -f /etc/pf.conf`.
- Release tarballs are reproducible and never replaced on a re-run.

## 0.2.0 — 2026-09-13

First release under `daemonless/fjord`. Default port moved from 9443 (Portainer's)
to **3567**.

### Engines
- AppJail engine driven by `appjail-director`: a stack is the director spec +
  `Makejail` + jail template materialized from the catalog's dbuild bundle;
  the per-service `appjail oci run` path is gone
- director runs serialized per stack, tolerant of "Project not found" on
  down, with HOME and PWD pinned (under rc(8) fjordd had HOME=/ and no PWD:
  director state landed in `/.director` and rebuilds failed on
  `${PWD}/template.conf`)
- appjail update checks: image digests via `buildah images`
- podman disk usage from `images` / `ps` / `volume ls` instead of
  `system df` (10–30 s on a busy host → under a second)
- no engine name is hard-coded outside the registry; stacks that predate
  multi-engine get their engine recorded once at startup

### Stacks
- **name-based ids**, compose/Portainer style: a new stack's id is the slug
  of its name (`radarr`, `radarr-2`), so containers, networks and volumes
  read `radarr_radarr_1` / `radarr_default`. Deleting a stack frees its
  name; a reinstall picks up the named volumes it kept. Existing numbered
  stacks are untouched
- Down/Up sweep "external" storage records in the stack's own namespace,
  so a half-finished podman rm can't block the next `compose up`
- app icon copied into the stack dir at install and served from there:
  survives the app leaving the catalog or the catalog being removed
- a draft stack can be renamed before Create
- engine picker on new stacks; director / Makejail editor tabs; engine
  mark (podman seal, jail bars) in the sidebar, header chip and list
- Resources tab shares the NFS/SMB folder form with folder sets; volume
  create/ensure/delete act on the stack's engine, not the default one
- start-on-boot skips stacks already running

### App store & install
- catalog carries `architectures`; the store hides apps not built for the
  host (either direction) with a toggle; Install stays disabled for them
- multi-service repos (affine, authelia, grimmory, onlyoffice, paperless,
  sparkyfitness, woodpecker) derived as stacks
- `latest` train picked first; optional ports; `devel`/edge never chosen
- pre-flight probes the EXPOSE ports of `network_mode: host` services (two
  stacks each shipping a host-network postgres collided on 5432 and
  crash-looped with nothing to say) and names the container holding a port
- missing bind-mount dirs are created chowned to PUID/PGID anywhere on the
  host (podman created them root-owned and the app couldn't write)
- wizard path hints wait for the App data location (a race fell back to
  `/containers`)
- the daemonless catalog is added on first run, removable, re-addable
- version trains: per-major channels (`8.6-pkg-latest`) stay separate; an
  arch-suffixed pin belongs to its channel

### Host
- readiness checks carry *why* and a copy-paste shell fix; pf fix generated
  from `/etc/pf.conf`; new `pf anchors for AppJail` check; data root
  auto-created
- setup wizard: Engine step; Re-check; catalog list with icons
- `/api/about` reports hostname; the tab title is `fjord - <host>`
- "unknown" update state is shown with its reason

### Removed
- container-mode `Containerfile` (parked; host install is the supported path)

## 0.1.1 — 2026-09-11
- BSD-2-Clause license
- CI publishes a release with a vendored source tarball and a FreeBSD/amd64
  binary on `v*` tags

## 0.1.0 — 2026-08-10 (tech preview)

First release. fjord is a web UI + daemon (`fjordd`) for running compose
stacks on FreeBSD (and, by design, Linux) — an app store, stack lifecycle
management, storage, and host diagnostics in one static Go binary.

**Tech preview:** there is no authentication. Run it on a trusted LAN only;
anyone who can reach the port can run containers as root on the host.

### Stacks
- Create, edit, start/stop/restart, delete compose stacks; apply-on-save flow
- Live status from the engine (real published ports, not the saved compose);
  on appjail the app inside the jail is probed, so a crash-looping service
  shows as partial, not green
- Merged/filterable container logs; interactive shell (WS + xterm, resize,
  heavy-output safe, exec sessions reaped); per-stack groups in the sidebar
- Start-on-boot restore of running stacks
- Preflight: port-conflict detection (containers *and* host processes) and
  auto-provisioned bind-mount dirs; a refused bring-up still saves the stack
  and lands you on it
- Stop and Delete confirm inline in the stack header, not in a modal; Delete
  refuses when containers survive the stop instead of orphaning them
- Stack names are validated on every route (no path traversal, no undeletable
  names)

### Engines
- Runtime-neutral engine interface; **podman** (podman-compose + libpod
  socket) and **appjail** (`appjail oci`, jails from OCI images) both work,
  side by side; each stack is bound to one engine at install
- `org.freebsd.jail.*` compose annotations become jail parameters on appjail
  (e.g. `allow.mlock` for .NET apps) via `appjail-config`
- A stack whose engine is unavailable reports that instead of being rerouted;
  a host with no engine still serves the UI and the System page

### App store
- Catalog-driven install wizard, three levels: name + anything with no sane
  default; **Options** (version/channel, App data location, folders, ports);
  **Advanced** (engine, env, custom tag, own IP). A one-line summary states
  the defaults being accepted
- Multi-service stacks (immich & co.); app-detail view; arch-filtered versions
- Multiple catalogs, merged and badged by origin; `FJORD_CATALOG_URL`
  bootstraps a first entry

### Storage
- **App data**: an ordered list of locations (default
  `<fjord root>/containers`); every app gets `<location>/<app>/…`; the
  wizard offers a pick when there is more than one. Existing folders are
  adopted, never re-chowned
- **Folder sets**: named sets of host folders, offered as "Add folder set…"
  on any host-path field at install and in Resources → Storage. Every
  host-path field is a folder list: several folders mount as sub-folders of
  the app's mount point, named uniquely
- **Remote folders**: `nfs://server/export` and `smb://user@server/share`
  entries become named volumes created on first use. SMB passwords are stored
  root-only on the host (`/etc/nsmb.conf` block on FreeBSD, credentials file
  on Linux), never in the compose or podman metadata
- `{{stack}}` / `{{appdata}}` placeholders in any host path
- Named volumes: create local / NFS / SMB from the Volumes page, attach from
  Resources → Storage, in-use guard, macvlan attach with static IP at install

### Plugins
- Engines are toggled per host; the **Homelab** plugin adds preset folder
  sets (Movies, TV, Music, Downloads, Books, Photos) that matching install
  fields pick up automatically. Off = plain folder sets

### Updates
- Fleet-wide update badges (digest drift + version upgrades), cached
  server-side and pruned when a stack is deleted; per-stack check,
  change-version with digest pin/unpin
- Registry digests via HEAD (no Docker Hub pull-quota burn); tag lists follow
  pagination; `${VAR}` image refs resolved against the stack `.env`

### Host doctor / System page
- `/api/setup` + System page: engine binaries, podman socket liveness,
  pf rdr-anchor presence, data-root writability — each with a copyable fix;
  jail/container mode detection
- Storage cleanup (prune) with the external containers that pin images
  (buildah builds, appjail jail sources) called out

### Daemon
- Single static binary, embedded UI, env-only configuration (`FJORD_*`)
- Settings persisted atomically in `settings.json` (App data, folder sets,
  catalogs, engines, plugins); legacy keys migrate on load
- `/api/about`, Settings page with Storage / Plugins / Catalogs tabs mirrored
  in the URL
- FreeBSD rc.d service (`packaging/fjordd.rc`) with boot-PATH handling

### Known limitations
- No authentication (see above). Also no CSRF/Origin checks yet
- FreeBSD's built-in SMB client speaks SMB1 only, so `smb://` folders and
  volumes work against modern servers only from Linux hosts; use NFS on
  FreeBSD
- appjail: no named volumes, networks, or update digests yet; remote folders
  need the podman engine
- Editing a folder set does not update stacks already installed from it
