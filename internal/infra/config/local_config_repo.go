package config

import (
	"context"
	"fmt"
	"sync"

	domainconfig "cloudpvp-matcher/internal/domain/config"
)

// LocalConfigRepository 基于内存中的 map 实现 ConfigRepository 端口。
// 配置由启动时从 config.yaml 或 Apollo 加载后注入。
type LocalConfigRepository struct {
	mu      sync.RWMutex
	configs map[domainconfig.GameMode]*domainconfig.MatchConfig
}

// 编译期检查接口实现
var _ domainconfig.ConfigRepository = (*LocalConfigRepository)(nil)

// NewLocalConfigRepository 创建本地配置仓储。
func NewLocalConfigRepository(configs []*domainconfig.MatchConfig) *LocalConfigRepository {
	m := make(map[domainconfig.GameMode]*domainconfig.MatchConfig, len(configs))
	for _, cfg := range configs {
		m[cfg.GameMode] = cfg
	}
	return &LocalConfigRepository{configs: m}
}

// GetMatchConfig 根据游戏模式获取匹配配置。
func (r *LocalConfigRepository) GetMatchConfig(ctx context.Context, mode domainconfig.GameMode) (*domainconfig.MatchConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.configs[mode]
	if !ok {
		return nil, fmt.Errorf("local config repo: config not found for mode %s", mode)
	}
	return cfg, nil
}

// GetAllMatchConfigs 返回所有已注册的匹配配置。
func (r *LocalConfigRepository) GetAllMatchConfigs(ctx context.Context) ([]*domainconfig.MatchConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domainconfig.MatchConfig, 0, len(r.configs))
	for _, cfg := range r.configs {
		result = append(result, cfg)
	}
	return result, nil
}
