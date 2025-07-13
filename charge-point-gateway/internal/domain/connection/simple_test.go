package connection

import (
	"testing"
)

func TestSimpleConnection(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)
	if conn == nil {
		t.Fatal("Connection should not be nil")
	}
	
	if conn.ID != "test" {
		t.Errorf("Expected ID 'test', got '%s'", conn.ID)
	}
}

func TestSimpleMessageCounting(t *testing.T) {
	conn := NewConnection("test", "CP001", ConnectionTypeWebSocket, ProtocolVersionOCPP16)
	
	// 测试发送消息计数
	conn.IncrementMessagesSent(100)
	if conn.NetworkInfo.MessagesSent != 1 {
		t.Errorf("Expected MessagesSent 1, got %d", conn.NetworkInfo.MessagesSent)
	}
	if conn.NetworkInfo.BytesSent != 100 {
		t.Errorf("Expected BytesSent 100, got %d", conn.NetworkInfo.BytesSent)
	}
}
