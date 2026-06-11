package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	domainconfig "cloudpvp-matcher/internal/domain/config"
	"cloudpvp-matcher/internal/infra/cache"
	"cloudpvp-matcher/internal/infra/mq"

	"github.com/apolloconfig/agollo/v5"
)

type apolloEntry struct {
	namespace string
	key       string
}

var typeNameMap = map[reflect.Type]apolloEntry{
	reflect.TypeOf(mq.RabbitMQConfig{}):           {namespace: "cloudpvp.mq", key: "key"},
	reflect.TypeOf(cache.Config{}):                {namespace: "cloudpvp.redis", key: "key"},
	reflect.TypeOf([]*domainconfig.MatchConfig{}): {namespace: "application", key: "match_modes"},
}

// Get 按目标类型从 Apollo 读取强类型配置。
func Get[T any](client agollo.Client) (*T, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	entry, ok := typeNameMap[t]
	if !ok {
		return nil, fmt.Errorf("apollo config type not registered: %s", t)
	}

	configCache := client.GetConfigCache(entry.namespace)
	value, err := configCache.Get(entry.key)
	if err != nil {
		return nil, fmt.Errorf("apollo get %s/%s: %w", entry.namespace, entry.key, err)
	}

	data, err := configValueJSON(value)
	if err != nil {
		return nil, fmt.Errorf("apollo encode %s/%s: %w", entry.namespace, entry.key, err)
	}
	var res T
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// configValueJSON 将 Apollo 返回值统一转换为 JSON 字节。
func configValueJSON(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, fmt.Errorf("empty config value")
		}
		return []byte(trimmed), nil
	case []byte:
		return v, nil
	default:
		return json.Marshal(value)
	}
}
