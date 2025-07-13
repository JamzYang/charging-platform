package message

import "github.com/charging-platform/charge-point-gateway/internal/domain/events"

// EventProducer 定义了向消息队列发布统一业务事件的接口
type EventProducer interface {
	// PublishEvent 异步发布一个事件
	PublishEvent(event events.Event) error
	// Close 关闭生产者
	Close() error
}