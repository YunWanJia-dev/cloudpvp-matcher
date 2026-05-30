// Package config contains matchmaking configuration domain types.
package config

// GameMode 标识特定的游戏模式（如 "csgo/5v5/competitive"）。
type GameMode string

const (
	GameModeCSGO5v5 GameMode = "csgo/5v5/competitive"
)
