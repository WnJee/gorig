package wecom

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	configure "github.com/WnJee/gorig/utils/cofigure"
)

const AtAll = "@all"

// Notify sends a text message to Enterprise WeChat (WeCom) robot.
func Notify(atUser string, title, msg string) error {
	key := configure.GetString("notify.wecom.key")
	if key == "" {
		key = configure.GetString("notify.wecom.token")
	}
	if key == "" {
		return fmt.Errorf("wecom webhook key not configured (notify.wecom.key)")
	}

	webhookURL := key
	if !strings.HasPrefix(key, "http://") && !strings.HasPrefix(key, "https://") {
		webhookURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
	}

	hostname, _ := os.Hostname()
	fullTitle := fmt.Sprintf("[%s系统][%s环境]%s\nHostname:%s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title, hostname)

	content := fmt.Sprintf("%s\n%s", fullTitle, msg)

	textPayload := map[string]interface{}{
		"content": content,
	}

	if atUser != "" {
		if atUser == AtAll {
			textPayload["mentioned_list"] = []string{"@all"}
		} else {
			textPayload["mentioned_mobile_list"] = []string{atUser}
		}
	}

	payload := map[string]interface{}{
		"msgtype": "text",
		"text":    textPayload,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom send failed with status: %d", resp.StatusCode)
	}

	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.ErrCode != 0 {
		return fmt.Errorf("wecom api error: %d %s", res.ErrCode, res.ErrMsg)
	}

	return nil
}

// SendMarkdown sends a markdown message to Enterprise WeChat robot.
func SendMarkdown(title, markdownContent string) error {
	key := configure.GetString("notify.wecom.key")
	if key == "" {
		key = configure.GetString("notify.wecom.token")
	}
	if key == "" {
		return fmt.Errorf("wecom webhook key not configured")
	}

	webhookURL := key
	if !strings.HasPrefix(key, "http://") && !strings.HasPrefix(key, "https://") {
		webhookURL = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
	}

	hostname, _ := os.Hostname()
	content := fmt.Sprintf("### [%s系统][%s环境] %s\n> **Hostname**: %s\n\n%s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title, hostname, markdownContent)

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecom markdown send failed with status: %d", resp.StatusCode)
	}
	return nil
}

func PanicNotifyDefault(msg string) error {
	return PanicNotify(AtAll, msg)
}

func PanicNotify(atUser string, msg string) error {
	return Notify(atUser, "发生【错误！！！】", msg)
}

func ErrNotifyDefault(msg string) error {
	return ErrNotify(AtAll, msg)
}

func ErrNotify(atUser string, msg string) error {
	return Notify(atUser, "发生【异常】", msg)
}
