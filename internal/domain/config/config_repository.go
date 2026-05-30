package config

import "context"

// ConfigRepository 是从配置中心加载匹配规则的端口。
type ConfigRepository interface {
	GetMatchConfig(ctx context.Context, mode GameMode) (*MatchConfig, error)
	GetAllMatchConfigs(ctx context.Context) ([]*MatchConfig, error)
}
