package messagex

import (
	"github.com/WnJee/gorig/utils/errors"
	"testing"
)

func TestPrepareMessagePreservesIDAndNormalizesContent(t *testing.T) {
	original := &Message{
		ID:      "message-1",
		Content: map[string]interface{}{"count": 1},
	}

	prepared, err := prepareMessage(original, "group-1")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.ID != original.ID {
		t.Fatalf("expected stable message ID %q, got %q", original.ID, prepared.ID)
	}
	if prepared.TargetGroup != "group-1" {
		t.Fatalf("expected target group to be set, got %q", prepared.TargetGroup)
	}
	if _, ok := prepared.Content["count"].(float64); !ok {
		t.Fatalf("expected JSON-compatible numeric content, got %T", prepared.Content["count"])
	}
	prepared.Content["count"] = 2
	if original.Content["count"] != 1 {
		t.Fatal("preparing a message must not mutate the caller's content")
	}
}

func TestMessageSetValueInitializesContent(t *testing.T) {
	message := &Message{}
	message.SetValue("Name", "alice")
	if got := message.GetValue("name"); got != "alice" {
		t.Fatalf("expected stored value, got %v", got)
	}
}

func TestPublishRejectsNilMessage(t *testing.T) {
	if err := Publish("topic", nil); err == nil {
		t.Fatal("publishing a nil message must return an error")
	}
}

func TestRedisBrokerDoesNotSilentlyFallbackToLocal(t *testing.T) {
	broker := &SimpleMessageBroker{brokerType: Redis}
	if _, err := broker.Subscribe("topic", func(*Message) *errors.Error { return nil }); err == nil {
		t.Fatal("a Redis broker without a Redis store must reject subscriptions")
	}
}
