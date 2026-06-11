package config

import (
	"log"

	"github.com/apolloconfig/agollo/v5"
	"github.com/apolloconfig/agollo/v5/env/config"
)

// NewApolloClient 根据本地引导配置启动 Apollo 客户端。
func NewApolloClient(c *config.AppConfig) agollo.Client {
	client, err := agollo.StartWithConfig(func() (*config.AppConfig, error) {
		return c, nil
	})
	if err != nil {
		log.Panicf("start agollo error: %s", err)
	}

	return client
}
