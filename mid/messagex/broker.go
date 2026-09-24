package messagex

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/WnJee/gorig/utils/errors"
	"github.com/rs/xid"
	"strconv"
	"strings"
)

type MessageType string

type Message struct {
	Ctx         context.Context `json:"-"`
	ID          string
	GroupID     string
	TargetGroup string `json:"target_group,omitempty"`
	SubID       uint64
	Topic       string
	Retry       int
	Content     map[string]interface{}
}

type BrokerType int

const (
	Local    = iota
	RabbitMQ // RabbitMQ
	Redis    // Redis
)

// MessageBroker 定义了消息代理的行为。
type MessageBroker interface {
	Subscribe(topic string, handler func(message *Message) *errors.Error) (uint64, *errors.Error)
	SubscribeGroup(topic string, groupID string, handler func(message *Message) *errors.Error) (uint64, *errors.Error)  // 指定groupID
	SubscribeSeq(topic string, handler func(message *Message) *errors.Error, opts ...SeqOption) (uint64, *errors.Error) // 顺序消费，handler 同步执行
	ReplayDLQ(topic string, limit int) *errors.Error                                                                    // 将DLQ消息重新入队，limit<=0 表示全部
	UnSubscribe(topic string, subID uint64) *errors.Error
	Publish(topic string, message *Message) *errors.Error
	PublishGroup(topic string, groupID string, message *Message) *errors.Error // 指定groupID
	TopicList() []string
}

func (m *Message) GetValue(key string) interface{} {
	if m == nil {
		return nil
	}
	key = strings.ToLower(key)
	v, ok := m.Content[key]
	if !ok {
		return nil
	}
	return v
}

func (m *Message) GetValueInt64(key string) int64 {
	v := m.GetValue(key)
	if v == nil {
		return 0
	}
	switch v.(type) {
	case int64:
		return v.(int64)
	case int:
		return int64(v.(int))
	case float64:
		return int64(v.(float64))
	case json.Number:
		i, _ := v.(json.Number).Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(v.(string), 10, 64)
		return i
	}
	return 0
}

func (m *Message) GetValueFloat64(key string) float64 {
	v := m.GetValue(key)
	if v == nil {
		return 0
	}
	switch v.(type) {
	case float64:
		return v.(float64)
	case string:
		i, _ := strconv.ParseFloat(v.(string), 64)
		return i
	}
	return 0
}

func (m *Message) GetValueStr(key string) string {
	v := m.GetValue(key)
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (m *Message) SetValue(key string, value interface{}) {
	if m == nil {
		return
	}
	key = strings.ToLower(key)
	if m.Content == nil {
		m.Content = make(map[string]interface{})
	}
	m.Content[key] = value
}

// Bind decodes message content into a typed struct pointer.
func Bind[T any](m *Message) (*T, error) {
	if m == nil || m.Content == nil {
		return new(T), nil
	}
	b, err := json.Marshal(m.Content)
	if err != nil {
		return nil, err
	}
	result := new(T)
	if err := json.Unmarshal(b, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Message) DeepCopy() *Message {
	if m == nil {
		return nil
	}

	clone := &Message{
		ID:          m.ID,
		GroupID:     m.GroupID,
		TargetGroup: m.TargetGroup,
		Ctx:         m.Ctx,
		SubID:       m.SubID,
		Topic:       m.Topic,
		Retry:       m.Retry,
		Content:     nil,
	}

	// Deep copy Content map
	if m.Content != nil {
		clone.Content = make(map[string]interface{}, len(m.Content))
		for key, value := range m.Content {
			clone.Content[key] = deepCopyValue(value)
		}
	}

	return clone
}

// prepareMessage takes the caller-owned message snapshot used by both local
// and Redis brokers. JSON normalization keeps values consistent across the
// in-process and Redis transports (for example, all JSON numbers become
// float64), while the stable ID allows consumers to deduplicate retries.
func prepareMessage(message *Message, groupID string) (*Message, *errors.Error) {
	clone := message.DeepCopy()
	if clone.ID == "" {
		clone.ID = xid.New().String()
	}
	clone.TargetGroup = groupID
	if clone.Content == nil {
		return clone, nil
	}
	raw, err := json.Marshal(clone.Content)
	if err != nil {
		return nil, errors.Sys(fmt.Sprintf("message content encode error: %v", err))
	}
	content := make(map[string]interface{}, len(clone.Content))
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil, errors.Sys(fmt.Sprintf("message content decode error: %v", err))
	}
	clone.Content = content
	return clone, nil
}

func deepCopyValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			result[key] = deepCopyValue(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return value
	}
}

func (m *Message) LowerContentKey() {
	if m.Content != nil {
		for k, v := range m.Content {
			lk := strings.ToLower(k)
			m.Content[lk] = v
			if k != lk {
				delete(m.Content, k)
			}
		}
	}
}
