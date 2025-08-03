package message

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/charging-platform/charge-point-gateway/internal/domain/events"
	"github.com/charging-platform/charge-point-gateway/internal/metrics"
	"github.com/rs/zerolog/log"
)

// IntegrationEvent 集成事件格式（符合对接文档）
type IntegrationEvent struct {
	EventID       string      `json:"eventId"`
	EventType     string      `json:"eventType"`
	ChargePointID string      `json:"chargePointId"`
	GatewayID     string      `json:"gatewayId"`
	Timestamp     string      `json:"timestamp"`
	Payload       interface{} `json:"payload"`
}

// IntegrationEventConverter 集成事件转换器
type IntegrationEventConverter struct {
	gatewayID string
}

// NewIntegrationEventConverter 创建集成事件转换器
func NewIntegrationEventConverter(gatewayID string) *IntegrationEventConverter {
	return &IntegrationEventConverter{
		gatewayID: gatewayID,
	}
}

// ConvertToIntegrationFormat 将内部事件转换为集成事件格式
func (c *IntegrationEventConverter) ConvertToIntegrationFormat(event events.Event) *IntegrationEvent {
	return &IntegrationEvent{
		EventID:       event.GetID(),
		EventType:     c.mapEventType(event.GetType()),
		ChargePointID: event.GetChargePointID(),
		GatewayID:     c.gatewayID,
		Timestamp:     event.GetTimestamp().Format(time.RFC3339),
		Payload:       c.convertPayload(event),
	}
}

// mapEventType 映射内部事件类型到对接文档约定的事件类型
func (c *IntegrationEventConverter) mapEventType(internalType events.EventType) string {
	switch internalType {
	case events.EventTypeChargePointConnected:
		return "charge_point.connected"
	case events.EventTypeChargePointDisconnected:
		return "charge_point.disconnected"
	case events.EventTypeConnectorStatusChanged:
		return "connector.status_changed"
	case events.EventTypeTransactionStarted:
		return "transaction.started"
	case events.EventTypeMeterValuesReceived:
		return "transaction.meter_values"
	case events.EventTypeTransactionStopped:
		return "transaction.stopped"
	case events.EventTypeRemoteCommandExecuted:
		return "command.response"
	default:
		// 对于未映射的事件类型，保持原样
		return string(internalType)
	}
}

// convertPayload 转换事件载荷为对接文档约定的格式
func (c *IntegrationEventConverter) convertPayload(event events.Event) interface{} {
	switch e := event.(type) {
	case *events.ChargePointConnectedEvent:
		payload := map[string]interface{}{
			"model":  e.ChargePointInfo.Model,
			"vendor": e.ChargePointInfo.Vendor,
		}
		if e.ChargePointInfo.FirmwareVersion != nil {
			payload["firmwareVersion"] = *e.ChargePointInfo.FirmwareVersion
		}
		return payload
	case *events.ChargePointDisconnectedEvent:
		return map[string]interface{}{
			"reason": "tcp_connection_closed",
		}
	case *events.ConnectorStatusChangedEvent:
		payload := map[string]interface{}{
			"connectorId":    e.ConnectorInfo.ID,
			"status":         c.formatConnectorStatus(e.ConnectorInfo.Status),
			"previousStatus": c.formatConnectorStatus(e.PreviousStatus),
		}
		if e.ConnectorInfo.ErrorCode != nil {
			payload["errorCode"] = *e.ConnectorInfo.ErrorCode
		}
		return payload
	case *events.TransactionStartedEvent:
		return map[string]interface{}{
			"connectorId":   e.TransactionInfo.ConnectorID,
			"transactionId": e.TransactionInfo.ID,
			"idTag":         e.TransactionInfo.IdTag,
			"meterStartWh":  e.TransactionInfo.MeterStart,
		}
	case *events.MeterValuesReceivedEvent:
		// 转换电表值格式
		meterValues := make([]map[string]interface{}, 0, len(e.MeterValues))
		for _, mv := range e.MeterValues {
			sampledValue := map[string]interface{}{
				"value":     mv.Value,
				"measurand": c.mapMeterValueType(mv.Type),
			}
			if mv.Unit != nil {
				sampledValue["unit"] = *mv.Unit
			}

			meterValue := map[string]interface{}{
				"timestamp":    mv.Timestamp.Format(time.RFC3339),
				"sampledValue": sampledValue,
			}
			meterValues = append(meterValues, meterValue)
		}

		payload := map[string]interface{}{
			"connectorId": e.ConnectorID,
			"meterValues": meterValues,
		}
		if e.TransactionID != nil {
			payload["transactionId"] = *e.TransactionID
		}
		return payload
	case *events.TransactionStoppedEvent:
		payload := map[string]interface{}{
			"transactionId": e.TransactionInfo.ID,
		}
		if e.TransactionInfo.StopReason != nil {
			payload["reason"] = *e.TransactionInfo.StopReason
		}
		if e.TransactionInfo.MeterStop != nil {
			payload["meterStopWh"] = *e.TransactionInfo.MeterStop
		}
		if e.TransactionInfo.EndTime != nil {
			payload["stopTimestamp"] = e.TransactionInfo.EndTime.Format(time.RFC3339)
		}
		return payload
	default:
		// 对于其他事件类型，直接返回原始载荷
		return event.GetPayload()
	}
}

// mapMeterValueType 映射电表值类型到OCPP标准格式
func (c *IntegrationEventConverter) mapMeterValueType(valueType events.MeterValueType) string {
	switch valueType {
	case events.MeterValueTypeEnergyActiveImport:
		return "Energy.Active.Import.Register"
	case events.MeterValueTypePowerActiveImport:
		return "Power.Active.Import"
	case events.MeterValueTypeVoltage:
		return "Voltage"
	case events.MeterValueTypeCurrentImport:
		return "Current.Import"
	default:
		return string(valueType)
	}
}

// formatConnectorStatus 格式化连接器状态为对接文档约定的格式（首字母大写）
func (c *IntegrationEventConverter) formatConnectorStatus(status events.ConnectorStatus) string {
	switch status {
	case events.ConnectorStatusAvailable:
		return "Available"
	case events.ConnectorStatusPreparing:
		return "Preparing"
	case events.ConnectorStatusCharging:
		return "Charging"
	case events.ConnectorStatusSuspendedEVSE:
		return "SuspendedEVSE"
	case events.ConnectorStatusSuspendedEV:
		return "SuspendedEV"
	case events.ConnectorStatusFinishing:
		return "Finishing"
	case events.ConnectorStatusReserved:
		return "Reserved"
	case events.ConnectorStatusUnavailable:
		return "Unavailable"
	case events.ConnectorStatusFaulted:
		return "Faulted"
	default:
		// 对于未知状态，首字母大写
		statusStr := string(status)
		if len(statusStr) > 0 {
			return strings.ToUpper(statusStr[:1]) + statusStr[1:]
		}
		return statusStr
	}
}

type KafkaProducer struct {
	producer  sarama.AsyncProducer
	topic     string
	converter *IntegrationEventConverter
}

// NewKafkaProducer 创建一个新的 KafkaProducer
func NewKafkaProducer(brokers []string, topic string, gatewayID string) (*KafkaProducer, error) {
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
		producer:  producer,
		topic:     topic,
		converter: NewIntegrationEventConverter(gatewayID),
	}

	// 启动 goroutine 处理成功和失败的 Kafka 消息
	go kp.handleSuccesses()
	go kp.handleErrors()

	return kp, nil
}

func (p *KafkaProducer) PublishEvent(event events.Event) error {
	// 1. 转换为集成事件格式
	integrationEvent := p.converter.ConvertToIntegrationFormat(event)

	// 2. 序列化为 JSON
	eventData, err := json.Marshal(integrationEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal integration event to JSON: %w", err)
	}

	// 3. 创建 Kafka 消息
	msg := &sarama.ProducerMessage{
		Topic:    p.topic,
		Key:      sarama.StringEncoder(event.GetChargePointID()), // 使用充电桩ID作为Key，保证同一桩的消息落入同一分区
		Value:    sarama.ByteEncoder(eventData),
		Metadata: event,
	}

	// 4. 发送消息
	p.producer.Input() <- msg

	log.Debug().
		Str("eventId", integrationEvent.EventID).
		Str("eventType", integrationEvent.EventType).
		Str("chargePointId", integrationEvent.ChargePointID).
		Str("gatewayId", integrationEvent.GatewayID).
		Msg("Publishing integration event to Kafka")

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
		if event, ok := msg.Metadata.(events.Event); ok {
			metrics.EventsPublished.WithLabelValues(string(event.GetType())).Inc()
		}
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
