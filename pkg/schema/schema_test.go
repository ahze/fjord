package schema_test

import (
	"testing"

	"github.com/daemonless/fjord/pkg/schema"
)

var validManifest = []byte(`{
  "name": "radarr",
  "services": {
    "radarr": {
      "image": "ghcr.io/daemonless/radarr:latest"
    }
  },
  "x-fjord": {
    "version": "0.1",
    "info": {
      "id": "radarr",
      "name": "Radarr",
      "description": "Automated movie collection manager",
      "category": "Media Management",
      "class": "service",
      "icon": "https://daemonless.io/fjord/icons/radarr.svg",
      "web_port": "${WEB_PORT}"
    },
    "host": {
      "min_freebsd_version": "15.0"
    },
    "variables": [
      {
        "name": "WEB_PORT",
        "label": "Web UI",
        "type": "port",
        "default": "7878"
      },
      {
        "name": "CONFIG_DATA",
        "label": "Configuration directory",
        "type": "zfs_dataset",
        "default": "config",
        "host_permissions": {
          "uid": 1000,
          "gid": 1000,
          "mode": "755"
        }
      }
    ]
  }
}`)

var invalidManifestMissingInfo = []byte(`{
  "x-fjord": {
    "version": "0.1"
  }
}`)

var validCatalog = []byte(`{
  "catalog_name": "Daemonless Apps",
  "fjord_version": "0.1",
  "maintainer": "https://daemonless.io",
  "generated": "2026-08-08T12:00:00Z",
  "apps": [
    {
      "id": "radarr",
      "name": "Radarr",
      "category": "Media Management",
      "class": "service",
      "icon": "https://daemonless.io/fjord/icons/radarr.svg",
      "manifest_url": "https://daemonless.io/fjord/manifests/radarr.yaml",
      "version": "5.14.0",
      "variants": [
        {
          "id": "latest",
          "label": "Upstream binary",
          "default": true,
          "image": "ghcr.io/daemonless/radarr:latest"
        }
      ]
    }
  ]
}`)

var invalidCatalog = []byte(`{
  "catalog_name": "Daemonless Apps",
  "fjord_version": "0.1"
}`)

func TestValidateManifest(t *testing.T) {
	if err := schema.ValidateManifest(validManifest); err != nil {
		t.Fatalf("expected valid manifest to pass, got error: %v", err)
	}

	if err := schema.ValidateManifest(invalidManifestMissingInfo); err == nil {
		t.Fatalf("expected invalid manifest to fail, but it passed")
	}
}

func TestValidateCatalog(t *testing.T) {
	if err := schema.ValidateCatalog(validCatalog); err != nil {
		t.Fatalf("expected valid catalog to pass, got error: %v", err)
	}

	if err := schema.ValidateCatalog(invalidCatalog); err == nil {
		t.Fatalf("expected invalid catalog to fail, but it passed")
	}
}
