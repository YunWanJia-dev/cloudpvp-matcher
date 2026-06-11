package config

import (
	"encoding/json"
	"time"
)

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

// UnmarshalJSON 支持 Apollo 中的字符串时长配置，例如 "30s"、"5m"。
func (c *MatchConfig) UnmarshalJSON(data []byte) error {
	var dto matchConfigJSON
	if err := json.Unmarshal(data, &dto); err != nil {
		return err
	}

	c.GameMode = dto.GameMode
	c.TeamSize = dto.TeamSize
	c.TeamCount = dto.TeamCount
	c.NeedConfirm = dto.NeedConfirm
	c.ConfirmTimeout = time.Duration(dto.ConfirmTimeout)
	c.MatchTimeout = time.Duration(dto.MatchTimeout)
	return nil
}

type matchConfigJSON struct {
	GameMode       GameMode      `json:"game_mode"`
	TeamSize       int           `json:"team_size"`
	TeamCount      int           `json:"team_count"`
	NeedConfirm    bool          `json:"need_confirm"`
	ConfirmTimeout durationValue `json:"confirm_timeout"`
	MatchTimeout   durationValue `json:"match_timeout"`
}

type durationValue time.Duration

// UnmarshalJSON 支持字符串或秒数形式的时长配置。
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
