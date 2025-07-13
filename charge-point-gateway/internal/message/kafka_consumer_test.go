package message_test

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"sync"
	"testing"
	"time" // 导入 time 包

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/internal/logger"
	"github.com/charging-platform/charge-point-gateway/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSaramaConsumerGroup is a mock for our SaramaConsumerGroup interface
type MockSaramaConsumerGroup struct {
	mock.Mock
}

func (m *MockSaramaConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	args := m.Called(ctx, topics, handler)
	return args.Error(0)
}

func (m *MockSaramaConsumerGroup) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockSaramaConsumerGroupSession is a mock for sarama.ConsumerGroupSession
type MockSaramaConsumerGroupSession struct {
	mock.Mock
	ctx context.Context
}

func (m *MockSaramaConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	m.Called(msg, metadata)
}

func (m *MockSaramaConsumerGroupSession) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

// Add other methods if needed by the code under test, but keep it minimal.
func (m *MockSaramaConsumerGroupSession) Claims() map[string][]int32 { return nil }
func (m *MockSaramaConsumerGroupSession) MemberID() string           { return "" }
func (m *MockSaramaConsumerGroupSession) GenerationID() int32        { return 0 }
func (m *MockSaramaConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
}
func (m *MockSaramaConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
}
func (m *MockSaramaConsumerGroupSession) Commit() {}

// MockSaramaConsumerGroupClaim is a mock for sarama.ConsumerGroupClaim
type MockSaramaConsumerGroupClaim struct {
	msgChan chan *sarama.ConsumerMessage
	part    int32
}

func (m *MockSaramaConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return m.msgChan
}

func (m *MockSaramaConsumerGroupClaim) Partition() int32 {
	return m.part
}

func (m *MockSaramaConsumerGroupClaim) Topic() string             { return "test-topic" }
func (m *MockSaramaConsumerGroupClaim) InitialOffset() int64      { return 0 }
func (m *MockSaramaConsumerGroupClaim) HighWaterMarkOffset() int64 { return 0 }

func TestConsumeClaim(t *testing.T) {
	log, _ := logger.New(logger.DefaultConfig())
	podID := "test-pod-1"
	partitionNum := 10

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(podID))
	myPartition := int32(hasher.Sum32() % uint32(partitionNum))

	testCases := []struct {
		name                string
		claimPartition      int32
		messageValue        []byte
		expectHandlerCalled bool
		expectedCmd         *message.Command
		setupSessionMock    func(session *MockSaramaConsumerGroupSession)
	}{
		{
			name:                "should process message from correct partition",
			claimPartition:      myPartition,
			messageValue:        mustMarshal(t, &message.Command{ChargePointID: "CP001", CommandName: "Start"}),
			expectHandlerCalled: true,
			expectedCmd:         &message.Command{ChargePointID: "CP001", CommandName: "Start"},
			setupSessionMock: func(session *MockSaramaConsumerGroupSession) {
				session.On("MarkMessage", mock.Anything, "").Return()
			},
		},
		{
			name:                "should skip message from incorrect partition",
			claimPartition:      myPartition + 1,
			messageValue:        mustMarshal(t, &message.Command{ChargePointID: "CP002", CommandName: "Stop"}),
			expectHandlerCalled: false,
			expectedCmd:         nil,
			setupSessionMock:    func(session *MockSaramaConsumerGroupSession) {},
		},
		{
			name:                "should not process invalid json message but still mark it",
			claimPartition:      myPartition,
			messageValue:        []byte(`{"invalid": "json"`),
			expectHandlerCalled: false,
			expectedCmd:         nil,
			setupSessionMock: func(session *MockSaramaConsumerGroupSession) {
				// MarkMessage should still be called to advance offset even on error
				session.On("MarkMessage", mock.Anything, "").Return()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Setup ---
			var receivedCmd *message.Command
			var wg sync.WaitGroup
			if tc.expectHandlerCalled {
				wg.Add(1)
			}
			handler := func(cmd *message.Command) {
				receivedCmd = cmd
				wg.Done()
			}

			// This is the key change: We create the consumer for testing without a real connection.
			consumer := message.NewKafkaConsumerForTest(podID, partitionNum, log, handler)

			msgChan := make(chan *sarama.ConsumerMessage, 1)
			msgChan <- &sarama.ConsumerMessage{Value: tc.messageValue}
			close(msgChan)

			mockClaim := &MockSaramaConsumerGroupClaim{
				msgChan: msgChan,
				part:    tc.claimPartition,
			}

			mockSession := &MockSaramaConsumerGroupSession{}
			tc.setupSessionMock(mockSession)

			// --- Act ---
			err := consumer.ConsumeClaim(mockSession, mockClaim)

			// --- Assert ---
			assert.NoError(t, err)

			if tc.expectHandlerCalled {
				wg.Wait() // Ensure handler has time to be called
				assert.Equal(t, tc.expectedCmd, receivedCmd)
			} else {
				// Give a moment for any unexpected handler calls
				time.Sleep(20 * time.Millisecond)
				assert.Nil(t, receivedCmd)
			}

			mockSession.AssertExpectations(t)
		})
	}
}

func TestKafkaConsumerStartAndClose(t *testing.T) {
	topic := "test-topic"
	log, _ := logger.New(logger.DefaultConfig())

	// 使用 sarama 的 MockConsumerGroup
	mockConsumerGroup := new(MockSaramaConsumerGroup)

	// 预期 Consume 会被调用
	mockConsumerGroup.On("Consume", mock.Anything, []string{topic}, mock.Anything).Return(nil)

	// 使用依赖注入创建 KafkaConsumer
	consumer := message.NewKafkaConsumerWithGroup(mockConsumerGroup, topic, "test-pod", 1, log)

	var handlerCalled bool
	var handlerWg sync.WaitGroup
	handlerWg.Add(1)
	handler := func(cmd *message.Command) {
		assert.Equal(t, "CP001", cmd.ChargePointID)
		handlerCalled = true
		handlerWg.Done()
	}

	// 启动消费者
	err := consumer.Start(handler)
	assert.NoError(t, err)

	// 模拟 Kafka 发送消息
	_ = mustMarshal(t, &message.Command{ChargePointID: "CP001", CommandName: "TestCommand"})
	// 模拟 Consume 循环的行为
	go func() {
		// 创建一个模拟的 session 和 claim
		mockSession := new(MockSaramaConsumerGroupSession)
		mockSession.On("MarkMessage", mock.Anything, "").Return()
		mockSession.On("Context").Return(context.Background())

		msgChan := make(chan *sarama.ConsumerMessage, 1)
		cmdBytes := mustMarshal(t, &message.Command{ChargePointID: "CP001", CommandName: "TestCommand"})
		msgChan <- &sarama.ConsumerMessage{Topic: topic, Partition: 0, Value: cmdBytes}
		close(msgChan)

		mockClaim := &MockSaramaConsumerGroupClaim{
			msgChan: msgChan,
			part:    0,
		}

		// 调用 ConsumeClaim
		err := consumer.ConsumeClaim(mockSession, mockClaim)
		assert.NoError(t, err)
	}()

	// 等待 handler 被调用
	handlerWg.Wait()
	assert.True(t, handlerCalled)

	// 关闭消费者
	mockConsumerGroup.On("Close").Return(nil) // 在调用 Close 之前设置预期
	err = consumer.Close()
	assert.NoError(t, err)

	// 验证 MockConsumerGroup 的所有预期都已满足
	mockConsumerGroup.AssertExpectations(t)
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	bytes, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	return bytes
}