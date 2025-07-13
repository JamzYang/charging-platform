package message

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/rs/zerolog/log"
)

type KafkaProducer struct {
	producer sarama.AsyncProducer
	topic    string
}

// NewKafkaProducer 创建一个新的 KafkaProducer
func NewKafkaProducer(brokers []string, topic string) (*KafkaProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForLocal       // 只等待本地确认
	config.Producer.Compression = sarama.CompressionSnappy   // 压缩
	config.Producer.Flush.Frequency = 500 * time.Millisecond // 刷新频率
	config.Producer.Return.Successes = true                  // 开启成功交付通知
	config.Producer.Return.Errors = true                     // 开启错误通知

	producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka async producer: %w", err)
	}

	kp := &KafkaProducer{
		producer: producer,
		topic:    topic,
	}

	// 启动 goroutine 处理成功和失败的 Kafka 消息
	go kp.handleSuccesses()
	go kp.handleErrors()

	return kp, nil
}

func (p *KafkaProducer) PublishEvent(event events.Event) error {
	// 1. 将 event 序列化为 JSON
	eventData, err := event.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	// 2. 创建 Kafka 消息
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(event.GetChargePointID()), // 使用充电桩ID作为Key，保证同一桩的消息落入同一分区
		Value: sarama.ByteEncoder(eventData),
	}

	// 3. 发送消息
	p.producer.Input() <- msg
	return nil
}

func (p *KafkaProducer) Close() error {
	if err := p.producer.Close(); err != nil {
		return fmt.Errorf("failed to close Kafka producer: %w", err)
	}
	return nil
}

func (p *KafkaProducer) handleSuccesses() {
	for msg := range p.producer.Successes() {
		log.Debug().
			Str("topic", msg.Topic).
			Str("key", string(msg.Key.(sarama.StringEncoder))).
			Msg("Kafka message sent successfully")
	}
}

func (p *KafkaProducer) handleErrors() {
	for err := range p.producer.Errors() {
		log.Error().
			Err(err).
			Str("topic", err.Msg.Topic).
			Str("key", string(err.Msg.Key.(sarama.StringEncoder))).
			Msg("Failed to send Kafka message")
	}
}