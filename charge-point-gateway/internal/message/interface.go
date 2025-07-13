package message

import (
	"context"
	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
)

// EventProducer 定义了向消息队列发布统一业务事件的接口
type EventProducer interface {
	// PublishEvent 异步发布一个事件
	PublishEvent(event events.Event) error
	// Close 关闭生产者
	Close() error
}

// Command 是下行指令的统一数据结构
type Command struct {
	ChargePointID string      `json:"chargePointId"`
	CommandName   string      `json:"commandName"`
	Payload       interface{} `json:"payload"`
}

// CommandHandler 是处理下行指令的函数类型
type CommandHandler func(cmd *Command)

// CommandConsumer 定义了消费下行指令的接口
type CommandConsumer interface {
	// Start 开始消费，并将接收到的指令传递给 handler
	Start(handler CommandHandler) error
	// Close 关闭消费者
	Close() error
}

// SaramaConsumerGroup is an interface that wraps sarama.ConsumerGroup, to allow for mocking.
type SaramaConsumerGroup interface {
	Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error
	Close() error
}