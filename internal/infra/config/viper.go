package config

import (
	apolloconfig "github.com/apolloconfig/agollo/v5/env/config"
	"github.com/spf13/viper"
)

var v *viper.Viper

// AppConfig 是启动 Apollo 客户端所需的本地引导配置。
type AppConfig struct {
	Apollo *apolloconfig.AppConfig
}

type localAppConfig struct {
	Apollo struct {
		AppID             string `mapstructure:"app_id"`
		Cluster           string `mapstructure:"cluster"`
		Namespace         string `mapstructure:"namespace"`
		MetaAddr          string `mapstructure:"meta_addr"`
		Secret            string `mapstructure:"secret"`
		IsBackupConfig    bool   `mapstructure:"is_backup_config"`
		BackupConfigPath  string `mapstructure:"backup_config_path"`
		MustStart         bool   `mapstructure:"must_start"`
		SyncServerTimeout int    `mapstructure:"sync_server_timeout"`
	} `mapstructure:"apollo"`
}

// init 初始化独立的 Viper 实例，避免污染全局配置。
func init() {
	instance := viper.New()
	instance.AutomaticEnv()
	v = instance
}

// LoadLocalAppConfig 读取本地 Apollo 引导配置。
func LoadLocalAppConfig(path string) (*AppConfig, error) {
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var local localAppConfig
	if err := v.Unmarshal(&local); err != nil {
		return nil, err
	}

	return &AppConfig{
		Apollo: &apolloconfig.AppConfig{
			AppID:             local.Apollo.AppID,
			Cluster:           local.Apollo.Cluster,
			NamespaceName:     local.Apollo.Namespace,
			IP:                local.Apollo.MetaAddr,
			Secret:            local.Apollo.Secret,
			IsBackupConfig:    local.Apollo.IsBackupConfig,
			BackupConfigPath:  local.Apollo.BackupConfigPath,
			MustStart:         local.Apollo.MustStart,
			SyncServerTimeout: local.Apollo.SyncServerTimeout,
		},
	}, nil
}
