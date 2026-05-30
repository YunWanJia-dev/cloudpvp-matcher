package handler

import "context"

// MessageHandler 是消息处理函数类型。
type MessageHandler func(ctx context.Context, body []byte) error

// Consumer 定义入站消息消费端口。
type Consumer interface {
	Run(ctx context.Context) error
}
