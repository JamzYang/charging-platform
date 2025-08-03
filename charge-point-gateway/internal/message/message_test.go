package message

import (
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
)

// MockAsyncProducer 是 sarama.AsyncProducer 的 mock 实现
type MockAsyncProducer struct {
	mock.Mock
	input     chan *sarama.ProducerMessage
	successes chan *sarama.ProducerMessage
	errors    chan *sarama.ProducerError
}

func NewMockAsyncProducer() *MockAsyncProducer {
	return &MockAsyncProducer{
		input:     make(chan *sarama.ProducerMessage),
		successes: make(chan *sarama.ProducerMessage),
		errors:    make(chan *sarama.ProducerError),
	}
}

func (m *MockAsyncProducer) AsyncClose() {
	m.Called()
	close(m.input)
	close(m.successes)
	close(m.errors)
}

func (m *MockAsyncProducer) Close() error {
	args := m.Called()
	m.AsyncClose() // 确保异步通道关闭
	return args.Error(0)
}

func (m *MockAsyncProducer) Input() chan<- *sarama.ProducerMessage {
	return m.input
}

func (m *MockAsyncProducer) AbortTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAsyncProducer) Successes() <-chan *sarama.ProducerMessage {
	return m.successes
}

func (m *MockAsyncProducer) IsTransactional() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockAsyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	args := m.Called()
	return args.Get(0).(sarama.ProducerTxnStatusFlag)
}

func (m *MockAsyncProducer) BeginTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAsyncProducer) CommitTxn() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAsyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupID string) error {
	args := m.Called(offsets, groupID)
	return args.Error(0)
}

func (m *MockAsyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupID string, metadata *string) error {
	args := m.Called(msg, groupID, metadata)
	return args.Error(0)
}

func (m *MockAsyncProducer) Errors() <-chan *sarama.ProducerError {
	return m.errors
}

// UnserializableEvent 实现了 events.Event 接口，但其 ToJSON 方法总是返回错误
type UnserializableEvent struct {
	*events.BaseEvent
}

func (e *UnserializableEvent) GetPayload() interface{} {
	return nil
}

func (e *UnserializableEvent) ToJSON() ([]byte, error) {
	return nil, assert.AnError // 总是返回一个错误
}

// TestEventProducerInterface 验证 EventProducer 接口的存在
func TestEventProducerInterface(t *testing.T) {
	// 尝试将一个 nil 赋值给接口，如果接口定义不正确，这将导致编译错误
	var producer EventProducer
	var kafkaProducer *KafkaProducer // 假设 KafkaProducer 实现了 EventProducer
	producer = kafkaProducer
	assert.Nil(t, producer) // 确保赋值成功，但 producer 仍然是 nil
}

// TestNewKafkaProducer_Success 测试 NewKafkaProducer 函数的成功创建
func TestNewKafkaProducer_Success(t *testing.T) {
	// 模拟 Kafka broker 地址
	brokers := []string{"localhost:9092"}
	topic := "test-topic"
	gatewayID := "test-gateway-id"

	// NewKafkaProducer 应该成功创建，即使没有实际的 Kafka broker 运行
	// 因为 sarama 的 AsyncProducer 是延迟连接的
	producer, err := NewKafkaProducer(brokers, topic, gatewayID)
	assert.NoError(t, err, "Expected no error when creating producer")
	assert.NotNil(t, producer, "Expected producer to be created")
	assert.Equal(t, topic, producer.topic)
	assert.NotNil(t, producer.converter)
	assert.Equal(t, gatewayID, producer.converter.gatewayID)

	// 清理资源
	if producer != nil {
		producer.Close()
	}
}

// TestPublishEvent_Failure 编写一个失败的测试，用于测试 PublishEvent 方法
func TestPublishEvent_Failure(t *testing.T) {
	mockProducer := NewMockAsyncProducer()
	mockProducer.On("Input").Return(make(chan *sarama.ProducerMessage)) // 模拟 Input channel

	kp := &KafkaProducer{
		producer:  mockProducer,
		topic:     "test-topic",
		converter: NewIntegrationEventConverter("test-gateway"),
	}

	badEvent := &UnserializableEvent{
		BaseEvent: events.NewBaseEvent(events.EventType("BadEventType"), "CP001", events.EventSeverityError, events.Metadata{}),
	}

	err := kp.PublishEvent(badEvent)
	assert.Error(t, err, "Expected an error when event serialization fails")
}

// TestClose_Failure 编写一个失败的测试，用于测试 Close 方法
func TestClose_Failure(t *testing.T) {
	mockProducer := NewMockAsyncProducer()
	mockProducer.On("Close").Return(assert.AnError) // 模拟 Close 返回错误
	mockProducer.On("AsyncClose").Return(nil)       // 模拟 AsyncClose

	kp := &KafkaProducer{
		producer: mockProducer,
		topic:    "test-topic",
	}

	err := kp.Close()
	assert.Error(t, err, "Expected an error when producer close fails")
	mockProducer.AssertExpectations(t)
}
