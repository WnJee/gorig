package httpx

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jom-io/gorig/utils/errors"
	"github.com/jom-io/gorig/utils/logger"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var client atomic.Pointer[http.Client]
var timeOut = 120 * time.Second

func init() {
	client.Store(&http.Client{Timeout: timeOut})
}

func getClient() *http.Client {
	if current := client.Load(); current != nil {
		return current
	}
	defaultClient := &http.Client{Timeout: timeOut}
	if client.CompareAndSwap(nil, defaultClient) {
		return defaultClient
	}
	return client.Load()
}

func SetTimeOutTmp(t time.Duration) {
	if t <= 0 {
		return
	}
	current := getClient()
	temporary := *current
	temporary.Timeout = t
	client.Store(&temporary)
	time.AfterFunc(t, func() {
		client.CompareAndSwap(&temporary, current)
	})
}

func buildURL(baseURL string, params map[string]string) (string, *errors.Error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", errors.Sys("invalid request URL", err)
	}
	values := parsed.Query()
	for key, value := range params {
		values.Set(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func Get(baseURL string, params map[string]string) (resp string, err *errors.Error) {
	reqURL, buildErr := buildURL(baseURL, params)
	if buildErr != nil {
		return "", buildErr
	}

	response, httpErr := getClient().Get(reqURL)
	if httpErr != nil {
		return "", errors.Sys("http.Get error", httpErr)
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("ioutil.ReadAll error: %v", readErr.Error()))
	}

	return string(body), nil
}

func GetHeader(baseURL string, params map[string]string, header map[string]string) (resp string, err *errors.Error) {
	reqURL, buildErr := buildURL(baseURL, params)
	if buildErr != nil {
		return "", buildErr
	}

	req, reqErr := http.NewRequest("GET", reqURL, nil)
	if reqErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.NewRequest error: %v", reqErr))
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	response, httpErr := getClient().Do(req)
	if httpErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.Do error: %v", httpErr))
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("ioutil.ReadAll error: %v", readErr))
	}

	return string(body), nil
}

func GetMap(baseURL string, params map[string]string) (map[string]interface{}, *errors.Error) {
	resp, err := Get(baseURL, params)
	if err != nil {
		return nil, err
	}
	return ParseJSON(resp), nil
}

func GetMapHeader(baseURL string, params map[string]string, header map[string]string) (map[string]interface{}, *errors.Error) {
	resp, err := GetHeader(baseURL, params, header)
	if err != nil {
		return nil, err
	}
	return ParseJSON(resp), nil
}

func PostForm(baseURL string, params map[string]string) (resp string, err *errors.Error) {
	values := url.Values{}
	for k, v := range params {
		values.Add(k, v)
	}

	response, httpErr := getClient().PostForm(baseURL, values)
	if httpErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.PostForm error: %v", httpErr.Error()))
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("ioutil.ReadAll error: %v", readErr.Error()))
	}

	return string(body), nil
}

func PostJSONResp(baseURL string, params interface{}) (resp string, err *errors.Error) {
	jsonData, marshalErr := json.Marshal(params)
	if marshalErr != nil {
		return "", errors.Sys(fmt.Sprintf("json.Marshal error: %v", marshalErr))
	}
	logger.Info(nil, "PostJSONResp", zap.String("url", baseURL))

	response, httpErr := getClient().Post(baseURL, "application/json", bytes.NewReader(jsonData))
	if httpErr != nil { // 注意这里的错误检查修正
		return "", errors.Sys(fmt.Sprintf("http.Post error: %v", httpErr))
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("io.ReadAll error: %v", readErr))
	}
	strBody := string(body)
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return strBody, errors.Sys(fmt.Sprintf("http.Post status:%v error: %v", response.StatusCode, string(body)))
	}
	return strBody, nil
}

func PostJSONRespHeader(baseURL string, params interface{}, header map[string]string) (resp string, err *errors.Error) {
	jsonData, marshalErr := json.Marshal(params)
	if marshalErr != nil {
		return "", errors.Sys(fmt.Sprintf("json.Marshal error: %v", marshalErr))
	}

	req, reqErr := http.NewRequest("POST", baseURL, bytes.NewReader(jsonData))
	if reqErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.NewRequest error: %v", reqErr))
	}
	req.Header.Set("Content-Type", "application/json")
	if header != nil {
		for k, v := range header {
			req.Header.Set(k, v)
		}
	}
	response, httpErr := getClient().Do(req)
	if httpErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.Do error: %v", httpErr))
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("io.ReadAll error: %v", readErr))
	}

	return string(body), nil
}

func PostJSON(baseURL string, params interface{}) (map[string]interface{}, *errors.Error) {
	respStr, err := PostJSONResp(baseURL, params)
	if err != nil {
		if respStr == "" {
			return nil, err
		} else {
			return ParseJSON(respStr), err
		}
	}
	return ParseJSON(respStr), nil
}

func PostJSONHeader(baseURL string, params interface{}, header map[string]string) (map[string]interface{}, *errors.Error) {
	respStr, err := PostJSONRespHeader(baseURL, params, header)
	if err != nil {
		if respStr == "" {
			return nil, err
		} else {
			return ParseJSON(respStr), err
		}
	}
	return ParseJSON(respStr), nil
}

func PostJSONByCtx(ctx *gin.Context, baseURL string, params interface{}) (map[string]interface{}, *errors.Error) {
	header := ctx.GetHeader("Authorization")
	auth := map[string]string{"Authorization": header}
	if header == "" {
		auth = nil
	}
	return PostJSONHeader(baseURL, params, auth)
}

func GetByCtx(ctx *gin.Context, baseURL string, params map[string]interface{}) (map[string]interface{}, *errors.Error) {
	header := ctx.GetHeader("Authorization")
	auth := map[string]string{"Authorization": header}
	strParams := make(map[string]string)
	for k, v := range params {
		strParams[k] = cast.ToString(v)
	}
	return GetMapHeader(baseURL, strParams, auth)
}

func PostXML(baseURL string, params map[string]string) (resp string, err *errors.Error) {
	xmlData := "<xml>"
	for k, v := range params {
		xmlData += fmt.Sprintf("<%s>%s</%s>", k, v, k)
	}
	xmlData += "</xml>"

	response, httpErr := getClient().Post(baseURL, "application/xml", bytes.NewReader([]byte(xmlData)))
	if httpErr != nil {
		return "", errors.Sys(fmt.Sprintf("http.Post error: %v", httpErr))
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return "", errors.Sys(fmt.Sprintf("io.ReadAll error: %v", readErr))
	}

	return string(body), nil
}

func ParseJSON(jsonStr string) map[string]interface{} {
	if jsonStr == "" {
		return nil
	}
	var result map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		logger.Error(nil, fmt.Sprintf("ParseJSON error: result=%v, err=%v", result, err))
		//panic(err)
	}
	return result
}

func ParseXML[T any](xmlStr string) (*T, *errors.Error) {
	var result T
	err := xml.Unmarshal([]byte(xmlStr), &result)
	if err != nil {
		return nil, errors.Sys("xml.Unmarshal error", err)
	}
	return &result, nil
}

// FetchImage fetches image from url
func FetchImage(url string) (imgData []byte, contentType, imgType string, error *errors.Error) {
	var imageType string
	if strings.Contains(url, ".") && len(url) > 4 {
		imageType = url[len(url)-4:]
	} else {
		return nil, "", imageType, errors.Sys("invalid image url")
	}
	if strings.Contains(imageType, "jpeg") || strings.Contains(imageType, "jpg") {
		contentType = "image/jpeg"
		imageType = ".jpg"
	}
	if strings.Contains(imageType, "png") {
		contentType = "image/png"
		imageType = ".png"
	}
	if strings.Contains(imageType, "gif") {
		contentType = "image/gif"
		imageType = ".gif"
	}
	if contentType == "" {
		contentType = "image/png"
		imageType = ".png"
	}

	response, httpErr := getClient().Get(url)
	if httpErr != nil {
		return nil, "", imageType, errors.Sys(fmt.Sprintf("http.Get error: %v", httpErr))
	}
	defer response.Body.Close()

	imgData, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return nil, "", imageType, errors.Sys(fmt.Sprintf("ioutil.ReadAll error: %v", readErr))
	}

	return imgData, contentType, imageType, nil
}
