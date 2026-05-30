package repository

import (
	"context"

	"cloudpvp-matcher/internal/domain/valueobject"
)

// ConfigRepository 是从配置中心加载匹配规则的端口。
type ConfigRepository interface {
	GetMatchConfig(ctx context.Context, mode valueobject.GameMode) (*valueobject.MatchConfig, error)
	GetAllMatchConfigs(ctx context.Context) ([]*valueobject.MatchConfig, error)
}
