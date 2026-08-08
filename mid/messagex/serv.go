package messagex

import (
	"context"
	"encoding/json"
	"github.com/jom-io/gorig/utils/errors"
	"github.com/jom-io/gorig/utils/logger"
	"github.com/jom-io/gorig/utils/sys"
	"github.com/rs/xid"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"reflect"
	"sync"
)

type MessageService struct {
	BrokerType BrokerType
	Broker     MessageBroker
}

var brokerTypeMap = map[BrokerType]MessageBroker{}
var brokerTypeMu sync.Mutex

func GetDef() *MessageService {
	return get(Local)
}

func Ins(brokerType BrokerType) *MessageService {
	return get(brokerType)
}

func get(brokerType BrokerType) *MessageService {
	brokerTypeMu.Lock()
	defer brokerTypeMu.Unlock()
	var broker MessageBroker

	if brokerTypeMap[brokerType] != nil {
		broker = brokerTypeMap[brokerType]
	} else {
		switch brokerType {
		case Local:
			broker = NewSimple()
		case RabbitMQ:
		// broker = NewRabbitMQMessageBroker()
		case Redis:
			broker = NewSimpleByType(Redis)
		default:
			panic("Unsupported broker type")
		}
		brokerTypeMap[brokerType] = broker
	}

	return &MessageService{
		BrokerType: brokerType,
		Broker:     broker,
	}
}

func getTopicStr(topic any) string {
	if topic == nil {
		return ""
	}
	if _, ok := topic.(string); !ok {
		topicValue := reflect.ValueOf(topic)
		topicType := topicValue.Type()
		if topicType.ConvertibleTo(reflect.TypeOf("")) {
			return topicValue.Convert(reflect.TypeOf("")).Interface().(string)
		}
		return ""
	} else {
		return topic.(string)
	}
}

func RegisterTopic(topic any, handler func(message *Message) *errors.Error) (uint64, *errors.Error) {
	return Ins(Local).RegisterTopic(topic, handler)
}

func RegisterTopicSeq(topic any, handler func(message *Message) *errors.Error, opts ...SeqOption) (uint64, *errors.Error) {
	return Ins(Local).RegisterTopicSeq(topic, handler, opts...)
}

func UnSubscribe(topic any, subID uint64) *errors.Error {
	return Ins(Local).UnRegisterTopic(topic, subID)
}

func (s *MessageService) RegisterTopic(topic any, handler func(message *Message) *errors.Error) (uint64, *errors.Error) {
	topicStr := getTopicStr(topic)
	subId, e := Ins(s.BrokerType).Broker.Subscribe(topicStr, handler)
	sys.Info(" # Reg Topic: ", topic, " # SubID: ", subId, " # BrokerType: ", s.BrokerType)
	if e != nil {
		logger.Error(nil, "Registering topic failed", zap.String("topic", topicStr), zap.Error(e))
	}
	return subId, e
}

func (s *MessageService) RegisterTopicSeq(topic any, handler func(message *Message) *errors.Error, opts ...SeqOption) (uint64, *errors.Error) {
	topicStr := getTopicStr(topic)
	subId, e := Ins(s.BrokerType).Broker.SubscribeSeq(topicStr, handler, opts...)
	sys.Info(" # Reg Topic Seq: ", topic, " # SubID: ", subId, " # BrokerType: ", s.BrokerType)
	if e != nil {
		logger.Error(nil, "Registering topic (seq) failed", zap.String("topic", topicStr), zap.Error(e))
	}
	return subId, e
}

func ReplayDLQ(topic any, limit int, brokerType ...BrokerType) *errors.Error {
	if len(brokerType) == 0 {
		brokerType = []BrokerType{Local}
	}
	return Ins(brokerType[0]).ReplayDLQ(topic, limit)
}

func (s *MessageService) ReplayDLQ(topic any, limit int) *errors.Error {
	topicStr := getTopicStr(topic)
	return Ins(s.BrokerType).Broker.ReplayDLQ(topicStr, limit)
}

func (s *MessageService) UnRegisterTopic(topic any, subID uint64) *errors.Error {
	topicStr := getTopicStr(topic)
	sys.Info(" # UnReg Topic: ", topic, " # SubID: ", subID, " # BrokerType: ", s.BrokerType)
	return Ins(s.BrokerType).Broker.UnSubscribe(topicStr, subID)
}

func (s *MessageService) Publish(ctx context.Context, topic any, message *Message) (error *errors.Error) {
	if message == nil {
		message = new(Message)
	}
	topicStr := getTopicStr(topic)
	if topicStr == "" {
		return errors.Verify("topic cannot be empty")
	}
	if topicStr != MsgStartup {
		sys.Info(" # Publish Topic: ", topicStr)
		logger.Info(ctx, "Publishing message", zap.String("group_id", message.GroupID), zap.String("topic", topicStr))
	}

	error = Ins(s.BrokerType).Broker.Publish(topicStr, message)
	if error != nil {
		logger.Error(ctx, "Publishing message failed", zap.String("topic", topicStr), zap.Error(error))
	}
	return
}

func (s *MessageService) PublishNewMsg(ctx context.Context, topic any, content any, groupId ...string) {
	if ctx == nil {
		ctx = context.Background()
	}
	defer func() {
		if r := recover(); r != nil {
			logger.DPanic(ctx, "PublishNewMsg panic", zap.Any("recover", r))
		}
	}()
	topicStr := getTopicStr(topic)
	if topicStr == "" {
		logger.Error(ctx, "PublishNewMsg: topic is empty")
		return
	}
	gid := xid.New().String()
	if len(groupId) > 0 {
		gid = groupId[0]
	} else {
		if ctx != nil {
			gid = cast.ToString(logger.GetTraceID(ctx))
		}
	}
	msg := &Message{
		Ctx:     ctx,
		ID:      xid.New().String(),
		GroupID: gid,
		Topic:   topicStr,
		Content: ToMap(content),
	}
	msg.LowerContentKey()
	Publish(msg.Topic, msg, s.BrokerType)
}

func Publish(topic any, message *Message, brokerType ...BrokerType) (error *errors.Error) {
	if len(brokerType) == 0 {
		brokerType = []BrokerType{Local}
	}
	return Ins(brokerType[0]).Publish(message.Ctx, topic, message)
}

func PublishWithCtx(ctx context.Context, topic any, message *Message) *errors.Error {
	topicStr := getTopicStr(topic)
	if message == nil {
		message = &Message{}
	}
	message.Ctx = ctx
	return Publish(topicStr, message)
}

func PublishNewMsg[T any](ctx context.Context, topic any, content T, groupId ...string) {
	Ins(Local).PublishNewMsg(ctx, topic, content, groupId...)
}

const (
	MsgStartup = "messagex.startup"
)

// ToMap converts a struct to a map[string]interface{} where the keys are the struct's field names
// and the values are the respective field values.
// Note: This function only works with structs and will return nil for non-struct parameters.
func ToMap(param interface{}) map[string]interface{} {
	// Return nil if the parameter is nil
	if param == nil {
		return nil
	}
	data, err := json.Marshal(param)
	if err != nil {
		return nil
	}
	result := make(map[string]interface{})
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	return result
}
