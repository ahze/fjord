package podman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

// ImageRepoDigests inspects a locally-present image via the libpod REST API and
// returns its RepoDigests ("repo@sha256:..."). An image that isn't pulled yields
// an empty slice (404), not an error, so update detection can report "unknown".
func (b *Backend) ImageRepoDigests(ctx context.Context, ref string) ([]string, error) {
	u := "http://d/v4.0.0/libpod/images/" + url.PathEscape(ref) + "/json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("libpod images inspect: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // image not present locally
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libpod images inspect: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out struct {
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode image inspect: %w", err)
	}
	return out.RepoDigests, nil
}

// ImageExposedPorts returns a locally-present image's EXPOSE entries as
// "port/proto" keys ("5432/tcp"). Not pulled yet -> nil, nil. Pre-flight uses
// it for network_mode: host services, whose ports never appear in `ports:`.
func (b *Backend) ImageExposedPorts(ctx context.Context, ref string) ([]string, error) {
	u := "http://d/v4.0.0/libpod/images/" + url.PathEscape(ref) + "/json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("libpod images inspect: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("libpod images inspect: unexpected status %s", resp.Status)
	}
	var out struct {
		Config struct {
			ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		} `json:"Config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode image inspect: %w", err)
	}
	keys := make([]string, 0, len(out.Config.ExposedPorts))
	for k := range out.Config.ExposedPorts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}
