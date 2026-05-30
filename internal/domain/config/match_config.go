package config

import "time"

// MatchConfig 保存从配置中心加载的特定游戏模式的匹配规则。
type MatchConfig struct {
	GameMode       GameMode      `yaml:"game_mode" json:"game_mode"`
	TeamSize       int           `yaml:"team_size" json:"team_size"`
	TeamCount      int           `yaml:"team_count" json:"team_count"`
	NeedConfirm    bool          `yaml:"need_confirm" json:"need_confirm"`
	ConfirmTimeout time.Duration `yaml:"confirm_timeout" json:"confirm_timeout"`
	MatchTimeout   time.Duration `yaml:"match_timeout" json:"match_timeout"`
}

// TotalPlayers 返回一场对局所需的玩家总数。
func (c *MatchConfig) TotalPlayers() int {
	return c.TeamSize * c.TeamCount
}
