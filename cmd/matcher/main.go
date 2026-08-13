package main

import (
	"cloudpvp-matcher/internal/app"
	"context"
	"errors"
	"flag"
)

// main 解析启动参数并启动匹配服务。
func main() {
	configPath := flag.String("config", "", "path to local Apollo bootstrap config")
	flag.Parse()

	err := app.Run(context.Background(), app.Options{ConfigPath: *configPath})
	if err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}
