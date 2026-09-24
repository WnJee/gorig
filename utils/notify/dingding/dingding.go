package dingding

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	configure "github.com/WnJee/gorig/utils/cofigure"
)

// 配置需要通知的联系人
const (
	AtAll = "all"
)

func genSign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(h.Sum(nil)))
}

// SendMessage sends message to DingTalk robot with error return.
func SendMessage(atUser string, title, msg string) error {
	token := configure.GetString("notify.dingding.token")
	if token == "" {
		return fmt.Errorf("dingding token not configured (notify.dingding.token)")
	}

	webhookURL := token
	if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") {
		webhookURL = "https://oapi.dingtalk.com/robot/send?access_token=" + token
	}

	secret := configure.GetString("notify.dingding.secret")
	if secret != "" {
		ts := time.Now().UnixMilli()
		sign := genSign(secret, ts)
		sep := "&"
		if !strings.Contains(webhookURL, "?") {
			sep = "?"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", webhookURL, sep, strconv.FormatInt(ts, 10), sign)
	}

	hostname, _ := os.Hostname()
	fullTitle := fmt.Sprintf("[%s系统][%s环境]%s \nHostname:%s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title, hostname)

	message := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": fmt.Sprintf("%s\n%s", fullTitle, msg),
		},
	}

	if atUser != "" {
		if atUser != AtAll {
			message["at"] = map[string]interface{}{
				"atMobiles": []string{atUser},
				"isAtAll":   false,
			}
		} else {
			message["at"] = map[string]interface{}{
				"isAtAll": true,
			}
		}
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(messageBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message to DingTalk, status: %d", resp.StatusCode)
	}

	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.ErrCode != 0 {
		return fmt.Errorf("dingtalk api error: %d %s", res.ErrCode, res.ErrMsg)
	}
	return nil
}

// SendMarkdown sends a markdown message to DingTalk.
func SendMarkdown(atUser, title, markdownContent string) error {
	token := configure.GetString("notify.dingding.token")
	if token == "" {
		return fmt.Errorf("dingding token not configured")
	}

	webhookURL := token
	if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") {
		webhookURL = "https://oapi.dingtalk.com/robot/send?access_token=" + token
	}

	secret := configure.GetString("notify.dingding.secret")
	if secret != "" {
		ts := time.Now().UnixMilli()
		sign := genSign(secret, ts)
		sep := "&"
		if !strings.Contains(webhookURL, "?") {
			sep = "?"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", webhookURL, sep, strconv.FormatInt(ts, 10), sign)
	}

	hostname, _ := os.Hostname()
	fullTitle := fmt.Sprintf("[%s系统][%s环境] %s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title)

	body := fmt.Sprintf("### %s\n> **Hostname**: %s\n\n%s", fullTitle, hostname, markdownContent)

	message := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  body,
		},
	}

	if atUser != "" {
		if atUser != AtAll {
			message["at"] = map[string]interface{}{
				"atMobiles": []string{atUser},
				"isAtAll":   false,
			}
		} else {
			message["at"] = map[string]interface{}{
				"isAtAll": true,
			}
		}
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(messageBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send markdown to DingTalk, status: %d", resp.StatusCode)
	}
	return nil
}

// Notify maintains backward compatibility with original DingTalk function signature.
func Notify(atUser string, title, msg string) {
	_ = SendMessage(atUser, title, msg)
}

func PanicNotifyDefault(msg string) {
	PanicNotify(AtAll, msg)
}

func PanicNotify(atUser string, msg string) {
	Notify(atUser, "发生【错误！！！】", msg)
}

func ErrNotifyDefault(msg string) {
	ErrNotify(AtAll, msg)
}

func ErrNotify(atUser string, msg string) {
	Notify(atUser, "发生【异常】", msg)
}
