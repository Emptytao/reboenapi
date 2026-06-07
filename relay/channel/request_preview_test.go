package channel

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type previewTestAdaptor struct{}
type previewMultipartAdaptor struct{}

func (a *previewTestAdaptor) Init(info *relaycommon.RelayInfo) {}
func (a *previewTestAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return "https://api.example.com/v1/messages", nil
}
func (a *previewTestAdaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	req.Set("Authorization", "Bearer top-secret")
	req.Set("X-Api-Key", "secret-key")
	req.Set("X-Trace-Id", "trace-123")
	req.Set("Content-Type", "application/json")
	return nil
}
func (a *previewTestAdaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return bytes.NewReader(nil), nil
}
func (a *previewTestAdaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return nil, nil
}
func (a *previewTestAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	return nil, nil
}
func (a *previewTestAdaptor) GetModelList() []string { return nil }
func (a *previewTestAdaptor) GetChannelName() string { return "preview-test" }
func (a *previewTestAdaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}
func (a *previewTestAdaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return request, nil
}

func (a *previewMultipartAdaptor) Init(info *relaycommon.RelayInfo) {}
func (a *previewMultipartAdaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return "https://api.example.com/v1/audio/transcriptions", nil
}
func (a *previewMultipartAdaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	req.Set("Authorization", "Bearer top-secret")
	req.Set("X-Trace-Id", "trace-123")
	return nil
}
func (a *previewMultipartAdaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return bytes.NewReader(nil), nil
}
func (a *previewMultipartAdaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return nil, nil
}
func (a *previewMultipartAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	return nil, nil
}
func (a *previewMultipartAdaptor) GetModelList() []string { return nil }
func (a *previewMultipartAdaptor) GetChannelName() string { return "preview-multipart-test" }
func (a *previewMultipartAdaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return request, nil
}
func (a *previewMultipartAdaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return request, nil
}

func TestTryWritePreviewFromAdaptorMasksSensitiveHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions?foo=bar", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Authorization", "Bearer downstream-secret")
	ctx.Set("role", appcommon.RoleAdminUser)

	info := &relaycommon.RelayInfo{
		RelayMode:             relayconstant.RelayModeChatCompletions,
		RequestURLPath:        "/v1/chat/completions",
		ClientRequestedStream: true,
		IsChannelPreviewMode:  true,
		OriginModelName:       "gpt-4o-mini",
		RequestConversionChain: []types.RelayFormat{
			types.RelayFormatOpenAI,
			types.RelayFormatClaude,
		},
		FinalRequestRelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         12,
			ChannelType:       1,
			ChannelBaseUrl:    "https://api.example.com",
			UpstreamModelName: "claude-3-5-sonnet",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				RequestPreviewModeEnabled: true,
			},
		},
	}

	handled, err := TryWritePreviewFromAdaptor(ctx, info, &previewTestAdaptor{}, bytes.NewBufferString(`{"prompt":"hello"}`))
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, IsRequestPreviewHandled(ctx))
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp map[string]any
	require.NoError(t, appcommon.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "channel_request_preview", resp["object"])

	channelPayload, ok := resp["channel"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "claude-3-5-sonnet", channelPayload["upstream_model"])

	relayPayload, ok := resp["relay"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "chat_completions", relayPayload["relay_mode"])
	require.Equal(t, "json", relayPayload["response_mode"])

	downstreamPayload := resp["downstream_request"].(map[string]any)
	downstreamRaw := downstreamPayload["raw_http"].(string)
	require.Contains(t, downstreamRaw, "POST /v1/chat/completions?foo=bar HTTP/1.1")
	require.Contains(t, downstreamRaw, "Authorization: Bearer downstream-secret")
	require.Contains(t, downstreamRaw, "Content-Type: application/json")
	require.Contains(t, downstreamRaw, `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)

	upstreamPayload := resp["upstream_request"].(map[string]any)
	upstreamRaw := upstreamPayload["raw_http"].(string)
	require.Contains(t, upstreamRaw, "POST /v1/messages HTTP/1.1")
	require.Contains(t, upstreamRaw, "Host: api.example.com")
	require.Contains(t, upstreamRaw, "Authorization: Bearer top-secret")
	require.Contains(t, upstreamRaw, "X-Api-Key: secret-key")
	require.Contains(t, upstreamRaw, "X-Trace-Id: trace-123")
	require.Contains(t, upstreamRaw, `{"prompt":"hello"}`)
}

func TestTryWritePreviewFromAdaptorSummarizesMultipartBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := "--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\nhello\r\n--boundary--\r\n"
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	ctx.Set("role", appcommon.RoleAdminUser)

	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeAudioTranscription,
		RequestURLPath:       "/v1/audio/transcriptions",
		IsChannelPreviewMode: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:         9,
			ChannelType:       1,
			ChannelBaseUrl:    "https://api.example.com",
			UpstreamModelName: "whisper-1",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				RequestPreviewModeEnabled: true,
			},
		},
	}

	handled, err := TryWritePreviewFromAdaptor(ctx, info, &previewMultipartAdaptor{}, bytes.NewBufferString(body))
	require.NoError(t, err)
	require.True(t, handled)

	var resp map[string]any
	require.NoError(t, appcommon.Unmarshal(recorder.Body.Bytes(), &resp))

	downstreamRaw := resp["downstream_request"].(map[string]any)["raw_http"].(string)
	require.Contains(t, downstreamRaw, "POST /v1/audio/transcriptions HTTP/1.1")
	require.Contains(t, downstreamRaw, "Content-Type: multipart/form-data; boundary=boundary")
	require.Contains(t, downstreamRaw, "[multipart body omitted in preview,")

	upstreamRaw := resp["upstream_request"].(map[string]any)["raw_http"].(string)
	require.Contains(t, upstreamRaw, "POST /v1/audio/transcriptions HTTP/1.1")
	require.Contains(t, upstreamRaw, "Content-Type: multipart/form-data; boundary=boundary")
	require.Contains(t, upstreamRaw, "[multipart body omitted in preview,")
}

func TestChannelOtherSettingsPreviewModeRoundTrip(t *testing.T) {
	t.Parallel()

	original := dto.ChannelOtherSettings{RequestPreviewModeEnabled: true}
	data, err := appcommon.Marshal(original)
	require.NoError(t, err)

	var decoded dto.ChannelOtherSettings
	require.NoError(t, appcommon.Unmarshal(data, &decoded))
	require.True(t, decoded.RequestPreviewModeEnabled)
}

func TestTryWritePreviewFromAdaptorReturnsUnavailableForCommonUser(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{
		RelayMode:            relayconstant.RelayModeChatCompletions,
		RequestURLPath:       "/v1/chat/completions",
		IsChannelPreviewMode: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				RequestPreviewModeEnabled: true,
			},
		},
	}

	handled, err := TryWritePreviewFromAdaptor(ctx, info, &previewTestAdaptor{}, bytes.NewBufferString(`{"prompt":"hello"}`))
	require.NoError(t, err)
	require.True(t, handled)
	require.True(t, IsRequestPreviewHandled(ctx))
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)

	var resp map[string]any
	require.NoError(t, appcommon.Unmarshal(recorder.Body.Bytes(), &resp))
	errorPayload := resp["error"].(map[string]any)
	require.Equal(t, "该模型正在调整，请稍后再试", errorPayload["message"])
}

func TestBodyTextFromLargeJSONReturnsFullJSON(t *testing.T) {
	t.Parallel()

	largePrompt := strings.Repeat("a", 1<<20+128)
	payload := []byte(`{"prompt":"` + largePrompt + `"}`)

	result := bodyTextFromBytes(payload, "application/json")
	require.Equal(t, string(payload), result)
}
