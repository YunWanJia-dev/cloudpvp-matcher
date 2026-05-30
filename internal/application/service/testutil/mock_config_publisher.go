package testutil

import (
	"context"

	"cloudpvp-matcher/internal/domain/entity"
	"cloudpvp-matcher/internal/domain/repository"
	"cloudpvp-matcher/internal/domain/valueobject"
)

// MockConfigRepository 基于内存的 ConfigRepository mock 实现。
type MockConfigRepository struct {
	configs map[valueobject.GameMode]*valueobject.MatchConfig
}

var _ repository.ConfigRepository = (*MockConfigRepository)(nil)

// NewMockConfigRepository 创建带一组配置的内存配置仓储。
func NewMockConfigRepository(configs []*valueobject.MatchConfig) *MockConfigRepository {
	m := make(map[valueobject.GameMode]*valueobject.MatchConfig, len(configs))
	for _, cfg := range configs {
		m[cfg.GameMode] = cfg
	}
	return &MockConfigRepository{configs: m}
}

func (m *MockConfigRepository) GetMatchConfig(ctx context.Context, mode valueobject.GameMode) (*valueobject.MatchConfig, error) {
	cfg, ok := m.configs[mode]
	if !ok {
		return nil, nil
	}
	return cfg, nil
}

func (m *MockConfigRepository) GetAllMatchConfigs(ctx context.Context) ([]*valueobject.MatchConfig, error) {
	result := make([]*valueobject.MatchConfig, 0, len(m.configs))
	for _, cfg := range m.configs {
		result = append(result, cfg)
	}
	return result, nil
}

// MockEventPublisher 记录发布调用而不实际发送消息的 mock 实现。
type MockEventPublisher struct {
	MatchResults     []*entity.Match
	ServerCreateReqs []*entity.Match
	ConfirmReqs      []*entity.Match
}

var _ repository.EventPublisher = (*MockEventPublisher)(nil)

// NewMockEventPublisher 创建空的事件发布者 mock。
func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{}
}

func (m *MockEventPublisher) PublishMatchResult(ctx context.Context, match *entity.Match) error {
	m.MatchResults = append(m.MatchResults, match)
	return nil
}

func (m *MockEventPublisher) PublishServerCreateRequest(ctx context.Context, match *entity.Match) error {
	m.ServerCreateReqs = append(m.ServerCreateReqs, match)
	return nil
}

func (m *MockEventPublisher) PublishConfirmRequest(ctx context.Context, match *entity.Match) error {
	m.ConfirmReqs = append(m.ConfirmReqs, match)
	return nil
}
