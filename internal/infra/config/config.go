// Package config loads local bootstrap configuration.
package config

import (
	"errors"
	"fmt"
	"strings"

	"cloudpvp-matcher/internal/infra/apollo"

	"github.com/spf13/viper"
)

// LoadApollo reads the local Apollo bootstrap config from config.yaml by
// default, or the explicit file path when provided.
//
// Environment variables can override scalar values. Both MATCHER_APOLLO_* names
// and legacy APOLLO_* names are supported.
func LoadApollo(path string) (apollo.Config, error) {
	v := viper.New()
	setDefaults(v)
	bindEnv(v)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if path != "" || !errors.As(err, &notFound) {
			return apollo.Config{}, fmt.Errorf("config: read config file: %w", err)
		}
	}

	cfg := apollo.Config{
		AppID:             v.GetString("apollo.app_id"),
		Cluster:           v.GetString("apollo.cluster"),
		Namespace:         v.GetString("apollo.namespace"),
		MetaAddr:          v.GetString("apollo.meta_addr"),
		Secret:            v.GetString("apollo.secret"),
		IsBackupConfig:    v.GetBool("apollo.is_backup_config"),
		BackupConfigPath:  v.GetString("apollo.backup_config_path"),
		MustStart:         v.GetBool("apollo.must_start"),
		SyncServerTimeout: v.GetInt("apollo.sync_server_timeout"),
	}

	if err := validateApollo(cfg); err != nil {
		return apollo.Config{}, err
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("apollo.app_id", "cloudpvp-matcher")
	v.SetDefault("apollo.cluster", "default")
	v.SetDefault("apollo.namespace", "application")
	v.SetDefault("apollo.meta_addr", "http://localhost:8080")
	v.SetDefault("apollo.secret", "")
	v.SetDefault("apollo.is_backup_config", true)
	v.SetDefault("apollo.backup_config_path", "")
	v.SetDefault("apollo.must_start", false)
	v.SetDefault("apollo.sync_server_timeout", 1)
}

func bindEnv(v *viper.Viper) {
	v.SetEnvPrefix("MATCHER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnvKeys(v, map[string][]string{
		"apollo.app_id":              {"APOLLO_APP_ID"},
		"apollo.cluster":             {"APOLLO_CLUSTER"},
		"apollo.namespace":           {"APOLLO_NAMESPACE"},
		"apollo.meta_addr":           {"APOLLO_META", "APOLLO_META_ADDR"},
		"apollo.secret":              {"APOLLO_SECRET"},
		"apollo.is_backup_config":    {"APOLLO_BACKUP_CONFIG"},
		"apollo.backup_config_path":  {"APOLLO_BACKUP_CONFIG_PATH"},
		"apollo.must_start":          {"APOLLO_MUST_START"},
		"apollo.sync_server_timeout": {"APOLLO_SYNC_SERVER_TIMEOUT"},
	})
}

func bindEnvKeys(v *viper.Viper, keys map[string][]string) {
	for key, aliases := range keys {
		envNames := []string{"MATCHER_" + strings.ToUpper(strings.ReplaceAll(key, ".", "_"))}
		envNames = append(envNames, aliases...)
		args := append([]string{key}, envNames...)
		_ = v.BindEnv(args...)
	}
}

func validateApollo(cfg apollo.Config) error {
	if cfg.AppID == "" {
		return errors.New("config: apollo.app_id is required")
	}
	if cfg.Namespace == "" {
		return errors.New("config: apollo.namespace is required")
	}
	if cfg.MetaAddr == "" {
		return errors.New("config: apollo.meta_addr is required")
	}
	return nil
}
