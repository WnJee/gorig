package notify

import (
	"context"
	"fmt"
	"strings"

	configure "github.com/WnJee/gorig/utils/cofigure"
	"github.com/WnJee/gorig/utils/logger"
	"github.com/WnJee/gorig/utils/notify/dingding"
	"github.com/WnJee/gorig/utils/notify/email"
	"github.com/WnJee/gorig/utils/notify/feishu"
	"github.com/WnJee/gorig/utils/notify/wecom"
)

type Channel string

const (
	DingDing Channel = "dingding"
	WeCom    Channel = "wecom"
	Feishu   Channel = "feishu"
	Email    Channel = "email"
)

// Message defines the unified notification message structure.
type Message struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	AtUsers  []string `json:"at_users"`
	AtAll    bool     `json:"at_all"`
	IsHTML   bool     `json:"is_html"`
	ToEmails []string `json:"to_emails"`
}

// GetConfiguredChannels returns all active notification channels configured in configuration.
func GetConfiguredChannels() []Channel {
	channels := configure.GetStringSlice("notify.channels")
	if len(channels) > 0 {
		res := make([]Channel, 0, len(channels))
		for _, c := range channels {
			res = append(res, Channel(strings.ToLower(strings.TrimSpace(c))))
		}
		return res
	}

	// Auto-detect channels by checking if tokens/hosts are configured
	var detected []Channel
	if configure.GetString("notify.dingding.token") != "" {
		detected = append(detected, DingDing)
	}
	if configure.GetString("notify.wecom.key") != "" || configure.GetString("notify.wecom.token") != "" {
		detected = append(detected, WeCom)
	}
	if configure.GetString("notify.feishu.token") != "" || configure.GetString("notify.feishu.key") != "" {
		detected = append(detected, Feishu)
	}
	if configure.GetString("notify.email.host") != "" {
		detected = append(detected, Email)
	}
	return detected
}

// Send broadcasts a message to specified or all configured channels.
func Send(ctx context.Context, msg *Message, channels ...Channel) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	targetChannels := channels
	if len(targetChannels) == 0 {
		targetChannels = GetConfiguredChannels()
	}
	if len(targetChannels) == 0 {
		// Fallback to dingding default if available
		targetChannels = []Channel{DingDing}
	}

	var errs []string
	for _, ch := range targetChannels {
		var err error
		switch ch {
		case DingDing:
			atUser := ""
			if msg.AtAll {
				atUser = dingding.AtAll
			} else if len(msg.AtUsers) > 0 {
				atUser = msg.AtUsers[0]
			}
			err = dingding.SendMessage(atUser, msg.Title, msg.Content)
		case WeCom:
			atUser := ""
			if msg.AtAll {
				atUser = wecom.AtAll
			} else if len(msg.AtUsers) > 0 {
				atUser = msg.AtUsers[0]
			}
			err = wecom.Notify(atUser, msg.Title, msg.Content)
		case Feishu:
			err = feishu.Notify(msg.AtAll, msg.Title, msg.Content)
		case Email:
			err = email.Send(msg.ToEmails, msg.Title, msg.Content, msg.IsHTML)
		default:
			err = fmt.Errorf("unknown notification channel: %s", ch)
		}

		if err != nil {
			logger.Warnf(ctx, "notify channel '%s' send failed: %v", ch, err)
			errs = append(errs, fmt.Sprintf("%s: %v", ch, err))
		}
	}

	if len(errs) > 0 && len(errs) == len(targetChannels) {
		return fmt.Errorf("all notification channels failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SendDingTalk sends a message to DingTalk.
func SendDingTalk(ctx context.Context, title, content string, atUsers ...string) error {
	msg := &Message{
		Title:   title,
		Content: content,
		AtUsers: atUsers,
		AtAll:   len(atUsers) == 0,
	}
	return Send(ctx, msg, DingDing)
}

// SendWeCom sends a message to Enterprise WeChat.
func SendWeCom(ctx context.Context, title, content string, atUsers ...string) error {
	msg := &Message{
		Title:   title,
		Content: content,
		AtUsers: atUsers,
		AtAll:   len(atUsers) == 0,
	}
	return Send(ctx, msg, WeCom)
}

// SendFeishu sends a message to Feishu.
func SendFeishu(ctx context.Context, title, content string, atAll ...bool) error {
	all := true
	if len(atAll) > 0 {
		all = atAll[0]
	}
	msg := &Message{
		Title:   title,
		Content: content,
		AtAll:   all,
	}
	return Send(ctx, msg, Feishu)
}

// SendEmail sends an email.
func SendEmail(ctx context.Context, subject, body string, to ...string) error {
	msg := &Message{
		Title:    subject,
		Content:  body,
		ToEmails: to,
	}
	return Send(ctx, msg, Email)
}

// PanicNotify sends a high-priority panic alert to configured channels.
func PanicNotify(ctx context.Context, msg string, channels ...Channel) error {
	return Send(ctx, &Message{
		Title:   "发生【错误！！！】",
		Content: msg,
		AtAll:   true,
	}, channels...)
}

// ErrNotify sends an exception alert to configured channels.
func ErrNotify(ctx context.Context, msg string, channels ...Channel) error {
	return Send(ctx, &Message{
		Title:   "发生【异常】",
		Content: msg,
		AtAll:   true,
	}, channels...)
}
