package podman

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// SMBCredDir holds per-server credential files on Linux (cifs
// credentials=...). Set by the daemon to <fjordRoot>/smb; root-only.
var SMBCredDir = "/var/db/fjord/smb"

// nsmbConf is a var so tests can point it at a temp file.
var nsmbConf = "/etc/nsmb.conf"

const (
	nsmbBegin  = "# >>> fjord-managed SMB credentials (do not edit inside) >>>"
	nsmbEnd    = "# <<< fjord-managed SMB credentials <<<"
	nsmbNotice = "# Managed by fjordd: one [SERVER:USER] section per SMB volume/folder set.\n"
)

// StoreSMBCredentials records the password an smb:// volume mount will use.
// podman supports remote volumes (see Capabilities), so this is a no-op-free
// path here; the generic handler reaches it through the engine interface.
func (b *Backend) StoreSMBCredentials(server, username, password string) error {
	return ensureSMBCredentials(server, username, password)
}

// ensureSMBCredentials records the password mount_smbfs / mount.cifs will use
// for user@server, where each platform's mounter reads it from: a managed
// block in /etc/nsmb.conf on FreeBSD (obfuscated with smbutil crypt), a
// root-only credentials file on Linux. Passwords never appear in podman's
// volume options. An empty password keeps whatever is already stored; "guest"
// needs none.
func ensureSMBCredentials(server, username, password string) error {
	if password == "" {
		if strings.EqualFold(username, "guest") || hasSMBCredentials(server, username) {
			return nil
		}
		return fmt.Errorf("smb: no stored password for %s@%s -- enter one", username, server)
	}
	if runtime.GOOS == "freebsd" {
		return writeNsmbSection(server, username, password)
	}
	if err := os.MkdirAll(SMBCredDir, 0o700); err != nil {
		return err
	}
	body := "username=" + username + "\npassword=" + password + "\n"
	return os.WriteFile(smbCredFile(server, username), []byte(body), 0o600)
}

func smbCredFile(server, username string) string {
	return filepath.Join(SMBCredDir, strings.ToLower(server)+"@"+strings.ToLower(username)+".cred")
}

func hasSMBCredentials(server, username string) bool {
	if runtime.GOOS == "freebsd" {
		b, err := os.ReadFile(nsmbConf)
		return err == nil && strings.Contains(string(b), "["+nsmbSection(server, username)+"]")
	}
	_, err := os.Stat(smbCredFile(server, username))
	return err == nil
}

// nsmbSection is the [SERVER:USER] key; nsmb.conf wants them upper-case.
func nsmbSection(server, username string) string {
	return strings.ToUpper(server) + ":" + strings.ToUpper(username)
}

// writeNsmbSection (re)writes one [SERVER:USER] section inside the managed
// block of /etc/nsmb.conf, leaving anything outside the markers untouched.
// The [SERVER] section pins addr= so the name resolves without NetBIOS.
func writeNsmbSection(server, username, password string) error {
	obf := password
	if out, err := exec.Command("smbutil", "crypt", password).Output(); err == nil {
		if v := strings.TrimSpace(string(out)); strings.HasPrefix(v, "$$1") {
			obf = v
		}
	}
	existing, _ := os.ReadFile(nsmbConf)
	text := string(existing)
	head, block, tail := text, "", ""
	if i := strings.Index(text, nsmbBegin); i >= 0 {
		if j := strings.Index(text[i:], nsmbEnd); j >= 0 {
			head, block, tail = text[:i], text[i+len(nsmbBegin):i+j], text[i+j+len(nsmbEnd):]
		}
	}
	// Drop any previous section for this server/user, then append the new one.
	var kept []string
	skip := false
	for _, line := range strings.Split(block, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			skip = t == "["+nsmbSection(server, username)+"]" || t == "["+strings.ToUpper(server)+"]"
		}
		if !skip && t != "" && t != strings.TrimSpace(nsmbNotice) {
			kept = append(kept, line)
		}
	}
	kept = append(kept,
		"["+strings.ToUpper(server)+"]",
		"addr="+server,
		"["+nsmbSection(server, username)+"]",
		"password="+obf,
	)
	newBlock := nsmbBegin + "\n" + nsmbNotice + strings.Join(kept, "\n") + "\n" + nsmbEnd
	out := strings.TrimRight(head, "\n")
	if out != "" {
		out += "\n\n"
	}
	out += newBlock + "\n" + strings.TrimLeft(tail, "\n")
	tmp := nsmbConf + ".fjord.tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, nsmbConf)
}

// hostNames resolves a uid/gid to names for mount_smbfs -u/-g, which take
// names only. "" when the host has no such user/group (then the mount falls
// back to world-writable modes so the container's user can still write).
func hostNames(uid, gid int) (uname, gname string) {
	if u, err := user.LookupId(fmt.Sprint(uid)); err == nil {
		uname = u.Username
	}
	if g, err := user.LookupGroupId(fmt.Sprint(gid)); err == nil {
		gname = g.Name
	}
	return
}
