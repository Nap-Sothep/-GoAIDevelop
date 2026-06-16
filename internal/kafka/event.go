package kafka

import (
	"encoding/json"
	"time"
)

// EventType 事件类型
type EventType string

const (
	// UserCreated 用户创建事件
	UserCreated EventType = "user_created"
	// UserUpdated 用户更新事件
	UserUpdated EventType = "user_updated"
	// UserDeleted 用户删除事件
	UserDeleted EventType = "user_deleted"
	// SystemLog 系统日志事件
	SystemLog EventType = "system_log"
)

// Event 通用事件结构（规则K4: 带版本号支持Schema演进）
type Event struct {
	Version   int       `json:"version"`    // Schema版本号
	EventType EventType `json:"event_type"` // 事件类型
	Timestamp int64     `json:"timestamp"`  // 时间戳
	Data      any       `json:"data"`       // 事件数据
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, data any) *Event {
	return &Event{
		Version:   1, // 当前版本
		EventType: eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
}

// ToJSON 序列化为JSON
func (e *Event) ToJSON() ([]byte, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// FromJSON 从JSON反序列化
func FromJSON(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// UserData 用户事件数据
type UserData struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// LogData 日志事件数据
type LogData struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Service string `json:"service"`
}
