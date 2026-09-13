package podman

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// defaultSocket matches the packaged podman_service rc.d unit
// (root:operator 0770, see fjord.md architecture decisions).
const defaultSocket = "/var/run/podman/podman.sock"

// socketPath is the podman unix socket. Override with FJORD_PODMAN_SOCKET
// (useful for rootless/dev setups where the packaged default isn't running).
func socketPath() string {
	if s := os.Getenv("FJORD_PODMAN_SOCKET"); s != "" {
		return s
	}
	return defaultSocket
}

// SocketPath exposes the resolved socket path for diagnostics (pkg/doctor).
func SocketPath() string { return socketPath() }

// Ping verifies the libpod REST API is actually answering on the socket --
// distinguishing a live service from a stale socket file left by a service
// that predates a podman upgrade.
func Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://d/v4.0.0/libpod/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := newSocketClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

// newSocketClient returns an http.Client that talks to the libpod REST API
// over the podman unix socket.
func newSocketClient() *http.Client {
	sockPath := socketPath()
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sockPath)
			},
		},
		Timeout: 10 * time.Second,
	}
}
