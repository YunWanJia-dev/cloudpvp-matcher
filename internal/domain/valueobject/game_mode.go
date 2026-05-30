// Package valueobject 定义领域层使用的值对象。
package valueobject

// GameMode 标识特定的游戏模式（如 "csgo/5v5/competitive"）。
type GameMode string

const (
	GameModeCSGO5v5 GameMode = "csgo/5v5/competitive"
)
