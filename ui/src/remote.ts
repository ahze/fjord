// Remote folders (NFS/SMB): the one place that knows how a remote folder is
// spelled, how its SMB password is stored, and how it becomes a named volume.
// Shared by the folder-set editor (Settings, setup wizard), the install wizard
// and the stack Resources tab so they can't drift apart.

export type RemoteKind = 'nfs' | 'smb';

export type RemoteSpec = { kind: RemoteKind; server: string; path: string; user: string; password: string };

// "NFS" / "SMB" for a remote row, "" for a host path.
export const remoteKind = (p: string) => (/^(nfs|smb):\/\//i.exec(p || '')?.[1] || '').toUpperCase();

// Turn a filled-in form into the row that gets stored: nfs://server/export or
// smb://user@server/share. The SMB password never rides on the row -- it is
// stored on the host (root-only) keyed by server + user. Throws with a
// user-facing message on bad input or a failed credential store.
export async function buildRemoteRow(f: RemoteSpec): Promise<string> {
  const server = f.server.trim();
  const path = f.path.trim().replace(/^\/+|\/+$/g, '');
  if (!server || !path) {
    throw new Error(f.kind === 'nfs' ? 'NFS needs a server and export path' : 'SMB needs a server and share');
  }
  if (f.kind === 'nfs') return `nfs://${server}/${path}`;
  const user = f.user.trim() || 'guest';
  if (f.password) {
    const r = await fetch('/api/volumes/smb-credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ server, user, password: f.password }),
    });
    if (!r.ok) throw new Error((await r.text()).trim() || `HTTP ${r.status}`);
  }
  return `smb://${user}@${server}/${path}`;
}

// Resolve a remote row to the named volume that mounts it, creating the
// volume on first use. Returns the volume name. The volume lives on the
// engine of the stack that will mount it (not the host default), so callers
// working on a stack pass its id.
export async function ensureRemoteVolume(source: string, scope: { stack?: string } = {}): Promise<string> {
  const q = scope.stack ? `?stack=${encodeURIComponent(scope.stack)}` : '';
  const r = await fetch('/api/volumes/ensure' + q, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ source }),
  });
  if (!r.ok) throw new Error((await r.text()).trim() || `HTTP ${r.status}`);
  return (await r.json()).name;
}
