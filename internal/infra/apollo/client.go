// Package apollo wraps the Apollo config center client.
package apollo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloudpvp-matcher/internal/domain/valueobject"

	"github.com/apolloconfig/agollo/v5"
	"github.com/apolloconfig/agollo/v5/env/config"
)

const matchModesKey = "match_modes"

// ErrConfigNotFound indicates that the requested Apollo key does not exist in
// the namespace cache.
var ErrConfigNotFound = errors.New("apollo: config not found")

// Client Apollo 配置客户端接口。
type Client struct {
	client    agollo.Client
	namespace string
}

// Config Apollo 连接配置。
type Config struct {
	AppID             string `yaml:"app_id"`
	Cluster           string `yaml:"cluster"`
	Namespace         string `yaml:"namespace"`
	MetaAddr          string `yaml:"meta_addr"` // Apollo Meta Server address.
	Secret            string `yaml:"secret"`
	IsBackupConfig    bool   `yaml:"is_backup_config"`
	BackupConfigPath  string `yaml:"backup_config_path"`
	MustStart         bool   `yaml:"must_start"`
	SyncServerTimeout int    `yaml:"sync_server_timeout"`
}

// NewClient creates and starts an agollo Apollo client.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if cfg.AppID == "" {
		return nil, errors.New("apollo: app id is required")
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "application"
	}
	if cfg.Cluster == "" {
		cfg.Cluster = "default"
	}
	if cfg.MetaAddr == "" {
		return nil, errors.New("apollo: meta address is required")
	}

	appCfg := &config.AppConfig{
		AppID:             cfg.AppID,
		Cluster:           cfg.Cluster,
		IP:                cfg.MetaAddr,
		NamespaceName:     cfg.Namespace,
		Secret:            cfg.Secret,
		IsBackupConfig:    cfg.IsBackupConfig,
		BackupConfigPath:  cfg.BackupConfigPath,
		MustStart:         cfg.MustStart,
		SyncServerTimeout: cfg.SyncServerTimeout,
	}

	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return appCfg, nil
	})
	if err != nil {
		return nil, fmt.Errorf("apollo: start agollo client: %w", err)
	}

	if err := ctx.Err(); err != nil {
		client.Close()
		return nil, err
	}

	return &Client{client: client, namespace: cfg.Namespace}, nil
}

// GetString 根据配置 key 获取字符串配置值。
func (c *Client) GetString(namespace, key string, defaultValue string) string {
	value, ok := c.get(namespace, key)
	if !ok {
		return defaultValue
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return defaultValue
		}
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

// GetInt 根据配置 key 获取整数配置值。
func (c *Client) GetInt(namespace, key string, defaultValue int) int {
	value, ok := c.get(namespace, key)
	if !ok {
		return defaultValue
	}

	n, err := toInt(value)
	if err != nil {
		return defaultValue
	}
	return n
}

// GetBool reads a boolean value from Apollo.
func (c *Client) GetBool(namespace, key string, defaultValue bool) bool {
	value, ok := c.get(namespace, key)
	if !ok {
		return defaultValue
	}

	b, err := toBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

// GetMatchConfigs reads the match_modes JSON config from Apollo.
//
// The Apollo value is expected to be a JSON array equivalent to config.yaml:
// [{"game_mode":"csgo/5v5/competitive","team_size":5,...}].
func (c *Client) GetMatchConfigs(namespace string) ([]*valueobject.MatchConfig, error) {
	raw, ok := c.get(namespace, matchModesKey)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, matchModesKey)
	}
	return parseMatchConfigs(raw)
}

// Close 关闭 Apollo 客户端。
func (c *Client) Close() {
	if c == nil || c.client == nil {
		return
	}
	c.client.Close()
}

func (c *Client) get(namespace, key string) (interface{}, bool) {
	if c == nil || c.client == nil || key == "" {
		return nil, false
	}
	if namespace == "" {
		namespace = c.namespace
	}

	value, err := c.client.GetConfigCache(namespace).Get(key)
	if err != nil {
		return nil, false
	}
	return value, true
}

func parseMatchConfigs(raw interface{}) ([]*valueobject.MatchConfig, error) {
	data, err := rawToJSON(raw)
	if err != nil {
		return nil, err
	}

	var dtos []matchConfigDTO
	if err := json.Unmarshal(data, &dtos); err != nil {
		return nil, fmt.Errorf("apollo: unmarshal %s: %w", matchModesKey, err)
	}

	configs := make([]*valueobject.MatchConfig, 0, len(dtos))
	for _, dto := range dtos {
		cfg := &valueobject.MatchConfig{
			GameMode:       valueobject.GameMode(dto.GameMode),
			TeamSize:       dto.TeamSize,
			TeamCount:      dto.TeamCount,
			NeedConfirm:    dto.NeedConfirm,
			ConfirmTimeout: time.Duration(dto.ConfirmTimeout),
			MatchTimeout:   time.Duration(dto.MatchTimeout),
		}
		if err := validateMatchConfig(cfg); err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

func rawToJSON(raw interface{}) ([]byte, error) {
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, fmt.Errorf("apollo: %s is empty", matchModesKey)
		}
		return []byte(s), nil
	case []byte:
		if len(v) == 0 {
			return nil, fmt.Errorf("apollo: %s is empty", matchModesKey)
		}
		return v, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("apollo: marshal %s: %w", matchModesKey, err)
		}
		return data, nil
	}
}

func validateMatchConfig(cfg *valueobject.MatchConfig) error {
	if cfg.GameMode == "" {
		return errors.New("apollo: match config game_mode is required")
	}
	if cfg.TeamSize <= 0 {
		return fmt.Errorf("apollo: match config %s team_size must be positive", cfg.GameMode)
	}
	if cfg.TeamCount <= 0 {
		return fmt.Errorf("apollo: match config %s team_count must be positive", cfg.GameMode)
	}
	if cfg.ConfirmTimeout <= 0 {
		return fmt.Errorf("apollo: match config %s confirm_timeout must be positive", cfg.GameMode)
	}
	if cfg.MatchTimeout <= 0 {
		return fmt.Errorf("apollo: match config %s match_timeout must be positive", cfg.GameMode)
	}
	return nil
}

func toInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		return int(n), err
	case string:
		return strconv.Atoi(strings.TrimSpace(v))
	default:
		return 0, fmt.Errorf("unsupported int value type %T", value)
	}
}

func toBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(v))
	default:
		return false, fmt.Errorf("unsupported bool value type %T", value)
	}
}

type matchConfigDTO struct {
	GameMode       string        `json:"game_mode"`
	TeamSize       int           `json:"team_size"`
	TeamCount      int           `json:"team_count"`
	NeedConfirm    bool          `json:"need_confirm"`
	ConfirmTimeout durationValue `json:"confirm_timeout"`
	MatchTimeout   durationValue `json:"match_timeout"`
}

type durationValue time.Duration

func (d *durationValue) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		duration, err := time.ParseDuration(text)
		if err != nil {
			return err
		}
		*d = durationValue(duration)
		return nil
	}

	var seconds float64
	if err := json.Unmarshal(data, &seconds); err != nil {
		return err
	}
	*d = durationValue(time.Duration(seconds * float64(time.Second)))
	return nil
}
