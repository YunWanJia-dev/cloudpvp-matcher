// Package config 包含匹配配置相关的领域类型。
package config

// GameMode 标识特定的游戏模式（如 "CS2/5v5/competitive"）。
type GameMode string

const (
	GameModeCSGO5v5 GameMode = "CS2/5v5/competitive"
)
