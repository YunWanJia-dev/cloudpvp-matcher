package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadApolloReadsLocalConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
apollo:
  app_id: matcher-test
  cluster: beta
  namespace: matcher.yaml
  meta_addr: http://apollo.local:8080
  secret: test-secret
  is_backup_config: false
  backup_config_path: /tmp/apollo-backup
  must_start: true
  sync_server_timeout: 3
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadApollo(path)
	if err != nil {
		t.Fatalf("LoadApollo() error = %v", err)
	}

	if cfg.AppID != "matcher-test" {
		t.Fatalf("AppID = %q, want matcher-test", cfg.AppID)
	}
	if cfg.Cluster != "beta" {
		t.Fatalf("Cluster = %q, want beta", cfg.Cluster)
	}
	if cfg.Namespace != "matcher.yaml" {
		t.Fatalf("Namespace = %q, want matcher.yaml", cfg.Namespace)
	}
	if cfg.MetaAddr != "http://apollo.local:8080" {
		t.Fatalf("MetaAddr = %q, want http://apollo.local:8080", cfg.MetaAddr)
	}
	if cfg.Secret != "test-secret" {
		t.Fatalf("Secret = %q, want test-secret", cfg.Secret)
	}
	if cfg.IsBackupConfig {
		t.Fatal("IsBackupConfig = true, want false")
	}
	if cfg.BackupConfigPath != "/tmp/apollo-backup" {
		t.Fatalf("BackupConfigPath = %q, want /tmp/apollo-backup", cfg.BackupConfigPath)
	}
	if !cfg.MustStart {
		t.Fatal("MustStart = false, want true")
	}
	if cfg.SyncServerTimeout != 3 {
		t.Fatalf("SyncServerTimeout = %d, want 3", cfg.SyncServerTimeout)
	}
}

func TestLoadApolloEnvOverridesLocalConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
apollo:
  app_id: matcher-test
  cluster: default
  namespace: application
  meta_addr: http://apollo.local:8080
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("MATCHER_APOLLO_META_ADDR", "http://env-apollo.local:8080")
	t.Setenv("APOLLO_MUST_START", "true")

	cfg, err := LoadApollo(path)
	if err != nil {
		t.Fatalf("LoadApollo() error = %v", err)
	}

	if cfg.MetaAddr != "http://env-apollo.local:8080" {
		t.Fatalf("MetaAddr = %q, want env override", cfg.MetaAddr)
	}
	if !cfg.MustStart {
		t.Fatal("MustStart = false, want env override")
	}
}
