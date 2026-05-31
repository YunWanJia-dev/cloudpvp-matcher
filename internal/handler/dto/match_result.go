package dto

import "time"

// MatchResult 是发送给 cloudpvp-biz 的匹配结果消息体。
type MatchResult struct {
	MessageID string     `json:"message_id"`
	MatchID   string     `json:"match_id"`
	GameMode  string     `json:"game_mode"`
	Teams     []TeamInfo `json:"teams"`
	MatchedAt time.Time  `json:"matched_at"`
}

// TeamInfo 描述匹配结果中的一个队伍。
type TeamInfo struct {
	LobbyID  string       `json:"lobby_id,omitempty"`
	LobbyIDs []string     `json:"lobby_ids,omitempty"`
	Members  []MemberInfo `json:"members"`
}
