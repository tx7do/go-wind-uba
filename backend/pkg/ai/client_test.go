package ai

// 本文件测试 client.go 的客户端工厂（表行 → OpenAI 兼容客户端）。
// 纯离线：
//   - HTTP 依赖全部走 httptest.Server 假上游（真发起一次 chat completion，
//     校验请求路径与 Authorization 头的归属）；
//   - 默认 BaseURL / authToken 占位等无法从外部观测的配置，
//     通过反射读取 openai.Client 的未导出 config 字段断言。
//
// 不测：真实外呼、流式路径、provider 归属校验/密钥解密（在 service 层）。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readClientConfig 反射读取 openai.Client 未导出的 config 字段。
// go-openai 没有导出读取 BaseURL/authToken 的方法，工厂函数的默认值语义
// （云端默认官方地址、本地默认 11434、空 key 占位 none）只能这样观测。
// HTTPClient 字段是 HTTPDoer 接口且取自未导出字段，无法 Interface() 还原
// 具体类型，故以"是否为 nil / Timeout 纳秒数"两个可观测值返回。
func readClientConfig(t *testing.T, c *openai.Client) (baseURL, authToken, orgID string, httpClientNil bool, httpClientTimeoutNanos int64) {
	t.Helper()
	v := reflect.ValueOf(c).Elem().FieldByName("config")
	require.True(t, v.IsValid(), "openai.Client.config 字段不存在，go-openai 版本可能已变更，需更新本测试")
	baseURL = v.FieldByName("BaseURL").String()
	authToken = v.FieldByName("authToken").String()
	orgID = v.FieldByName("OrgID").String()
	hc := v.FieldByName("HTTPClient")
	httpClientNil = hc.IsNil()
	if !httpClientNil {
		// 动态类型应为 *http.Client（hc.Elem() 为指针值，再 Elem() 解引用到结构体）；
		// Timeout 为 time.Duration（int64 底层）。
		require.Equal(t, reflect.TypeOf(&http.Client{}), hc.Elem().Type(),
			"工厂注入的 HTTPClient 应为 *http.Client")
		httpClientTimeoutNanos = hc.Elem().Elem().FieldByName("Timeout").Int()
	}
	return
}

// fakeChatServer 起一个返回最小合法 chat completion 响应的假上游，
// 记录最近一次请求的方法/路径/Authorization。
type fakeChatServer struct {
	srv  *httptest.Server
	port int

	mu     sync.Mutex
	path   string
	auth   string
	method string
}

func newFakeChatServer(t *testing.T) *fakeChatServer {
	t.Helper()
	f := &fakeChatServer{}
	respBody, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-fake",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   "fake-model",
		"choices": []map[string]any{
			{
				"index":         0,
				"message":       map[string]any{"role": "assistant", "content": "pong"},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2,
		},
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	// 兜底 handler：BaseURL 可能带也可能不带 /v1 前缀（云端走调用方配置的
	// BaseUrl，本地由工厂拼 /v1），因此按"路径以 /chat/completions 结尾"
	// 判定命中，其余路径记下来并 404，帮助定位 URL 拼接错误。
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.path, f.auth, f.method = r.URL.Path, r.Header.Get("Authorization"), r.Method
		f.mu.Unlock()
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(respBody)
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	u, err := url.Parse(f.srv.URL)
	require.NoError(t, err)
	f.port, err = strconv.Atoi(u.Port())
	require.NoError(t, err)
	return f
}

func (f *fakeChatServer) seen() (path, auth, method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.path, f.auth, f.method
}

// doChatCompletion 发起一次最小 chat completion 调用（非流式）。
func doChatCompletion(t *testing.T, ctx context.Context, c *openai.Client) openai.ChatCompletionResponse {
	t.Helper()
	resp, err := c.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    "fake-model",
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "ping"}},
	})
	require.NoError(t, err, "假上游正常返回时调用必须成功")
	return resp
}

// ---------------------------------------------------------------------------
// 参数校验
// ---------------------------------------------------------------------------

// TestNewClient_NilConfig nil 配置必须报错（调用方传入解密失败后的空配置）。
func TestNewClient_NilConfig(t *testing.T) {
	c, err := NewClient(nil, "any-key")
	assert.Error(t, err)
	assert.Nil(t, c)
	assert.Contains(t, err.Error(), "nil")
}

// TestNewClient_UnsupportedModelType ModelType 不在 {1=LOCAL, 2=CLOUD} 内必须
// 报错：表行被污染时宁可拒绝建客户端也不能静默打到默认地址。
func TestNewClient_UnsupportedModelType(t *testing.T) {
	for _, mt := range []int32{0, 3, -1} {
		c, err := NewClient(&ClientConfig{ModelType: mt}, "key")
		assert.Error(t, err, "ModelType=%d 应报错", mt)
		assert.Nil(t, c)
		assert.Contains(t, err.Error(), "unsupported ai model type")
	}
}

// ---------------------------------------------------------------------------
// 云端（CLOUD）路径
// ---------------------------------------------------------------------------

// TestNewClient_Cloud_BaseUrl 云端显式 BaseUrl：必须真正打到该地址的
// /chat/completions，并携带解密后的 key 作为 Bearer 凭证。
func TestNewClient_Cloud_BaseUrl(t *testing.T) {
	fake := newFakeChatServer(t)

	c, err := NewClient(&ClientConfig{
		ModelType: int32(ModelTypeCloud),
		BaseUrl:   fake.srv.URL,
	}, "sk-decrypted")
	require.NoError(t, err)
	require.NotNil(t, c)

	resp := doChatCompletion(t, context.Background(), c)
	assert.Equal(t, "pong", resp.Choices[0].Message.Content)
	assert.Equal(t, "chatcmpl-fake", resp.ID)

	path, auth, method := fake.seen()
	assert.True(t, strings.HasSuffix(path, "/chat/completions"),
		"云端请求必须落在标准 chat completions 路径, got %q", path)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "Bearer sk-decrypted", auth, "云端必须携带解密后的密钥")
}

// TestNewClient_Cloud_Defaults 云端未给 BaseUrl 时保留 go-openai 官方默认
// （调用方配空 BaseUrl 不等于"打到错误内网"）；OrgID 透传；
// HTTPClient 必须被工厂显式替换为无超时客户端（流式长回复不被腰斩）。
func TestNewClient_Cloud_Defaults(t *testing.T) {
	c, err := NewClient(&ClientConfig{
		ModelType:    int32(ModelTypeCloud),
		Organization: "org-42",
	}, "sk-x")
	require.NoError(t, err)

	baseURL, authToken, orgID, httpClientNil, httpClientTimeout := readClientConfig(t, c)
	assert.Equal(t, "https://api.openai.com/v1", baseURL, "BaseUrl 为空应保持官方默认")
	assert.Equal(t, "org-42", orgID)
	assert.Equal(t, "sk-x", authToken)
	assert.False(t, httpClientNil, "工厂必须显式注入 HTTPClient 覆盖默认 30s Timeout")
	assert.Zero(t, httpClientTimeout, "注入的 http.Client 必须 Timeout=0，流式长回复才不被腰斩")
}

// ---------------------------------------------------------------------------
// 本地（LOCAL / Ollama）路径
// ---------------------------------------------------------------------------

// TestNewClient_Local_ExplicitHostPort 本地显式 host/port：URL 形如
// http://<host>:<port>/v1，且无鉴权语义生效（空 key → 占位 none）。
func TestNewClient_Local_ExplicitHostPort(t *testing.T) {
	fake := newFakeChatServer(t)

	c, err := NewClient(&ClientConfig{
		ModelType: int32(ModelTypeLocal),
		LocalHost: "127.0.0.1",
		LocalPort: int32(fake.port),
	}, "")
	require.NoError(t, err)

	resp := doChatCompletion(t, context.Background(), c)
	assert.Equal(t, "pong", resp.Choices[0].Message.Content)

	path, auth, _ := fake.seen()
	assert.True(t, strings.HasSuffix(path, "/v1/chat/completions"),
		"本地请求必须由工厂拼出 /v1/chat/completions, got %q", path)
	assert.Equal(t, "Bearer none", auth, "本地 Ollama 无鉴权，空 key 必须占位为 none")
}

// TestNewClient_Local_Defaults 本地未配置 host/port 时的默认值：
// localhost:11434（Ollama 官方默认端口）。
func TestNewClient_Local_Defaults(t *testing.T) {
	c, err := NewClient(&ClientConfig{ModelType: int32(ModelTypeLocal)}, "")
	require.NoError(t, err)

	baseURL, _, _, _, _ := readClientConfig(t, c)
	assert.Equal(t, "http://localhost:11434/v1", baseURL)
}

// TestNewClient_Local_NoPortSuffix 逗号/空格等脏 host 不做清洗（工厂只做
// 拼接翻译，校验在 service 层），但至少保证 /v1 后缀与 http scheme 存在。
func TestNewClient_Local_KeepsV1Suffix(t *testing.T) {
	c, err := NewClient(&ClientConfig{
		ModelType: int32(ModelTypeLocal),
		LocalHost: "ollama.internal",
		LocalPort: 9999,
	}, "")
	require.NoError(t, err)

	baseURL, _, _, _, _ := readClientConfig(t, c)
	assert.Equal(t, "http://ollama.internal:9999/v1", baseURL)
}
