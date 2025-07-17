package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
)

type KafkaConsumer struct {
	consumerGroup SaramaConsumerGroup // 使用我们定义的接口
	topic         string
	podID         string // 当前 Pod 的唯一标识
	partitionNum  int    // 主题的总分区数
	logger        *logger.Logger
	cancel        context.CancelFunc
	handler       CommandHandler // 新增：存储指令处理函数
}

// NewKafkaConsumer 初始化 KafkaConsumer
// 完整的实现将在后续步骤中完成
func NewKafkaConsumer(brokers []string, groupID, topic, podID string, partitionNum int, logger *logger.Logger) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	config.Consumer.Group.Session.Timeout = 10 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 3 * time.Second

	consumerGroup, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sarama consumer group: %w", err)
	}

	go func() {
		for err := range consumerGroup.Errors() {
			logger.Errorf("Sarama consumer group error: %v", err)
		}
	}()

	return NewKafkaConsumerWithGroup(consumerGroup, topic, podID, partitionNum, logger), nil
}

// NewKafkaConsumerWithGroup is an exported helper function for dependency injection in tests.
func NewKafkaConsumerWithGroup(group SaramaConsumerGroup, topic, podID string, partitionNum int, logger *logger.Logger) *KafkaConsumer {
	return &KafkaConsumer{
		consumerGroup: group,
		topic:         topic,
		podID:         podID,
		partitionNum:  partitionNum,
		logger:        logger,
	}
}

// Start 启动消费者组
func (c *KafkaConsumer) Start(handler CommandHandler) error {
	c.handler = handler // 将 handler 存储在结构体中

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		defer cancel() // 确保在 goroutine 退出时取消 context
		for {
			// `Consume` 会在一个循环中处理，直到 session 结束
			if err := c.consumerGroup.Consume(ctx, []string{c.topic}, c); err != nil {
				c.logger.Errorf("Error from Kafka consumer group: %v", err)
				// 如果 context 被取消，则退出循环
				if ctx.Err() != nil {
					c.logger.Infof("Kafka consumer context cancelled, stopping consumption.")
					return
				}
				// 否则，等待一段时间后重试
				time.Sleep(time.Second) // 避免在错误情况下快速重试导致 CPU 占用过高
			}
		}
	}()
	return nil
}

// Close 关闭消费者
func (c *KafkaConsumer) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.consumerGroup != nil {
		return c.consumerGroup.Close()
	}
	return nil
}

// -- sarama.ConsumerGroupHandler 接口实现 --

func (c *KafkaConsumer) Setup(sarama.ConsumerGroupSession) error {
	c.logger.Info("Kafka consumer group setup completed.")
	return nil
}

func (c *KafkaConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	c.logger.Info("Kafka consumer group cleanup completed.")
	return nil
}

// ConsumeClaim 是核心消费逻辑
func (c *KafkaConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.logger.Infof("Pod %s consuming messages from partition %d", c.podID, claim.Partition())

	for message := range claim.Messages() {
		// 总是标记消息，即使处理失败，以避免重复消费
		defer session.MarkMessage(message, "")

		// 1. 反序列化消息为 Command
		var cmd Command
		if err := json.Unmarshal(message.Value, &cmd); err != nil {
			c.logger.Errorf("Failed to unmarshal Kafka message: %v, message: %s", err, string(message.Value))
			continue // 跳过此消息，继续处理下一条
		}

		// 2. 调用 c.handler(cmd)
		c.handler(&cmd)

		c.logger.Debugf("Message consumed and marked: Topic=%s, Partition=%d, Offset=%d, Key=%s",
			message.Topic, message.Partition, message.Offset, string(message.Key))
	}
	return nil
}

// NewKafkaConsumerForTest 仅为测试目的创建消费者实例。
// 它不初始化一个真正的消费者组。
func NewKafkaConsumerForTest(podID string, partitionNum int, logger *logger.Logger, handler CommandHandler) *KafkaConsumer {
	return &KafkaConsumer{
		podID:        podID,
		partitionNum: partitionNum,
		logger:       logger,
		handler:      handler,
	}
}
