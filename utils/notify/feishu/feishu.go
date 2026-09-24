package feishu

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	configure "github.com/WnJee/gorig/utils/cofigure"
)

// genSign generates signature for Feishu webhook with secret.
func genSign(secret string, timestamp int64) (string, error) {
	stringToSign := fmt.Sprintf("%v\n%v", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	_, err := h.Write([]byte{})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// Notify sends a text message to Feishu / Lark robot.
func Notify(atAll bool, title, msg string) error {
	token := configure.GetString("notify.feishu.token")
	if token == "" {
		token = configure.GetString("notify.feishu.key")
	}
	if token == "" {
		return fmt.Errorf("feishu webhook token not configured (notify.feishu.token)")
	}

	webhookURL := token
	if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") {
		webhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/" + token
	}

	hostname, _ := os.Hostname()
	fullTitle := fmt.Sprintf("[%s系统][%s环境]%s\nHostname:%s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title, hostname)

	content := fmt.Sprintf("%s\n%s", fullTitle, msg)
	if atAll {
		content = "<at user_id=\"all\">所有人</at> " + content
	}

	payload := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}

	secret := configure.GetString("notify.feishu.secret")
	if secret != "" {
		ts := time.Now().Unix()
		sign, err := genSign(secret, ts)
		if err == nil {
			payload["timestamp"] = strconv.FormatInt(ts, 10)
			payload["sign"] = sign
		}
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
		return fmt.Errorf("feishu send failed with status: %d", resp.StatusCode)
	}

	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && res.Code != 0 {
		return fmt.Errorf("feishu api error: %d %s", res.Code, res.Msg)
	}

	return nil
}

// SendCard sends an interactive rich card to Feishu robot.
func SendCard(title, content string, atAll ...bool) error {
	token := configure.GetString("notify.feishu.token")
	if token == "" {
		token = configure.GetString("notify.feishu.key")
	}
	if token == "" {
		return fmt.Errorf("feishu webhook token not configured")
	}

	webhookURL := token
	if !strings.HasPrefix(token, "http://") && !strings.HasPrefix(token, "https://") {
		webhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/" + token
	}

	hostname, _ := os.Hostname()
	cardHeader := fmt.Sprintf("[%s系统][%s环境] %s",
		configure.GetString("sys.name"), configure.GetString("sys.mode"), title)

	elements := []interface{}{
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**主机**: %s\n**时间**: %s\n\n%s", hostname, time.Now().Format("2006-01-02 15:04:05"), content),
			},
		},
	}

	if len(atAll) > 0 && atAll[0] {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": "<at id=all></at>",
			},
		})
	}

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": cardHeader,
				},
				"template": "red",
			},
			"elements": elements,
		},
	}

	secret := configure.GetString("notify.feishu.secret")
	if secret != "" {
		ts := time.Now().Unix()
		sign, err := genSign(secret, ts)
		if err == nil {
			payload["timestamp"] = strconv.FormatInt(ts, 10)
			payload["sign"] = sign
		}
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
		return fmt.Errorf("feishu card send failed with status: %d", resp.StatusCode)
	}
	return nil
}

func PanicNotifyDefault(msg string) error {
	return PanicNotify(true, msg)
}

func PanicNotify(atAll bool, msg string) error {
	return Notify(atAll, "发生【错误！！！】", msg)
}

func ErrNotifyDefault(msg string) error {
	return ErrNotify(true, msg)
}

func ErrNotify(atAll bool, msg string) error {
	return Notify(atAll, "发生【异常】", msg)
}
