package ticket

// TicketStatus 表示匹配票据的生命周期状态。
type TicketStatus int

const (
	TicketStatusPending    TicketStatus = iota // 初始状态，等待入队
	TicketStatusMatching                       // 已进入匹配池，寻找对手
	TicketStatusMatched                        // 已找到对手
	TicketStatusConfirming                     // 等待玩家确认中
	TicketStatusConfirmed                      // 所有玩家已确认
	TicketStatusCancelled                      // 用户或系统取消
	TicketStatusTimedOut                       // 超出匹配超时时间
)

func (s TicketStatus) String() string {
	switch s {
	case TicketStatusPending:
		return "pending"
	case TicketStatusMatching:
		return "matching"
	case TicketStatusMatched:
		return "matched"
	case TicketStatusConfirming:
		return "confirming"
	case TicketStatusConfirmed:
		return "confirmed"
	case TicketStatusCancelled:
		return "cancelled"
	case TicketStatusTimedOut:
		return "timed_out"
	default:
		return "unknown"
	}
}
