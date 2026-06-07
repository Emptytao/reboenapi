package channel

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	channelRequestPreviewHandledKey = "channel_request_preview_handled"
)

type previewBodyPayload struct {
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`
	JSON        any    `json:"json,omitempty"`
	Text        string `json:"text,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

type previewRequestPayload struct {
	Method  string              `json:"method"`
	Path    string              `json:"path,omitempty"`
	Query   map[string][]string `json:"query,omitempty"`
	URL     string              `json:"url,omitempty"`
	Headers map[string]string   `json:"headers,omitempty"`
	Body    previewBodyPayload  `json:"body"`
}

type channelPreviewResponse struct {
	Object          string                `json:"object"`
	Channel         previewChannelPayload `json:"channel"`
	Relay           previewRelayPayload   `json:"relay"`
	Downstream      previewRequestPayload `json:"downstream_request"`
	UpstreamRequest previewRequestPayload `json:"upstream_request"`
}

type previewChannelPayload struct {
	ID                        int    `json:"id"`
	Type                      int    `json:"type"`
	BaseURL                   string `json:"base_url,omitempty"`
	OriginModel               string `json:"origin_model,omitempty"`
	UpstreamModel             string `json:"upstream_model,omitempty"`
	RequestPreviewModeEnabled bool   `json:"request_preview_mode_enabled"`
}

type previewRelayPayload struct {
	RequestPath            string   `json:"request_path,omitempty"`
	RelayMode              string   `json:"relay_mode"`
	ClientRequestedStream  bool     `json:"client_requested_stream"`
	ResponseMode           string   `json:"response_mode"`
	RequestConversionChain []string `json:"request_conversion_chain,omitempty"`
	FinalRequestRelayFmt   string   `json:"final_request_relay_format,omitempty"`
}

func IsRequestPreviewHandled(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, exists := c.Get(channelRequestPreviewHandledKey)
	if !exists {
		return false
	}
	handled, ok := value.(bool)
	return ok && handled
}

func TryWritePreviewFromAdaptor(c *gin.Context, info *relaycommon.RelayInfo, adaptor Adaptor, requestBody io.Reader) (bool, error) {
	if info == nil || !info.IsChannelPreviewMode || adaptor == nil {
		return false, nil
	}
	if !canRevealSensitivePreviewData(c) {
		return true, writePreviewUnavailableResponse(c, info)
	}
	req, err := buildPreviewRequestForAdaptor(c, info, adaptor, requestBody)
	if err != nil {
		return false, err
	}
	return true, writePreviewResponse(c, info, req)
}

func TryWritePreviewFromTaskAdaptor(c *gin.Context, info *relaycommon.RelayInfo, adaptor TaskAdaptor, requestBody io.Reader) (bool, error) {
	if info == nil || !info.IsChannelPreviewMode || adaptor == nil {
		return false, nil
	}
	if !canRevealSensitivePreviewData(c) {
		return true, writePreviewUnavailableResponse(c, info)
	}
	req, err := buildPreviewRequestForTaskAdaptor(c, info, adaptor, requestBody)
	if err != nil {
		return false, err
	}
	return true, writePreviewResponse(c, info, req)
}

func buildPreviewRequestForAdaptor(c *gin.Context, info *relaycommon.RelayInfo, adaptor Adaptor, requestBody io.Reader) (*http.Request, error) {
	fullRequestURL, err := adaptor.GetRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	applyUpstreamContentLength(req, info)
	if shouldPreviewAsFormRequest(c, info) {
		req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	}
	headers := req.Header
	if err := adaptor.SetupRequestHeader(c, &headers, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	headerOverride, err := ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	applyHeaderOverrideToRequest(req, headerOverride)
	return req, nil
}

func buildPreviewRequestForTaskAdaptor(c *gin.Context, info *relaycommon.RelayInfo, adaptor TaskAdaptor, requestBody io.Reader) (*http.Request, error) {
	fullRequestURL, err := adaptor.BuildRequestURL(info)
	if err != nil {
		return nil, fmt.Errorf("get request url failed: %w", err)
	}
	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	applyUpstreamContentLength(req, info)
	if err := adaptor.BuildRequestHeader(c, req, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	return req, nil
}

func writePreviewResponse(c *gin.Context, info *relaycommon.RelayInfo, upstreamReq *http.Request) error {
	if c == nil || c.Request == nil {
		return fmt.Errorf("missing request context")
	}
	downstreamBody, err := buildDownstreamPreviewBody(c)
	if err != nil {
		return err
	}
	upstreamBody, err := buildRequestBodyPayload(upstreamReq)
	if err != nil {
		return err
	}

	rawResp := channelPreviewResponse{
		Object: "channel_request_preview",
		Channel: previewChannelPayload{
			ID:                        info.ChannelId,
			Type:                      info.ChannelType,
			BaseURL:                   info.ChannelBaseUrl,
			OriginModel:               info.OriginModelName,
			UpstreamModel:             info.UpstreamModelName,
			RequestPreviewModeEnabled: info.ChannelOtherSettings.RequestPreviewModeEnabled,
		},
		Relay: previewRelayPayload{
			RequestPath:            info.RequestURLPath,
			RelayMode:              relayModeName(info.RelayMode),
			ClientRequestedStream:  info.ClientRequestedStream,
			ResponseMode:           "json",
			RequestConversionChain: relayFormatStrings(info.RequestConversionChain),
			FinalRequestRelayFmt:   string(info.GetFinalRequestRelayFormat()),
		},
		Downstream: previewRequestPayload{
			Method:  c.Request.Method,
			Path:    c.Request.URL.Path,
			Query:   cloneQueryValues(c.Request.URL.Query()),
			Headers: flattenHeader(c.Request.Header),
			Body:    downstreamBody,
		},
		UpstreamRequest: previewRequestPayload{
			Method:  upstreamReq.Method,
			URL:     upstreamReq.URL.String(),
			Headers: flattenHeader(upstreamReq.Header, upstreamReq.Host),
			Body:    upstreamBody,
		},
	}

	payloadBytes, err := appcommon.Marshal(rawResp)
	if err == nil {
		model.RecordRequestPreviewLog(c, info.UserId, model.RecordRequestPreviewLogParams{
			ChannelId:             info.ChannelId,
			ChannelType:           info.ChannelType,
			RequestPath:           info.RequestURLPath,
			RelayMode:             relayModeName(info.RelayMode),
			OriginModelName:       info.OriginModelName,
			UpstreamModelName:     info.UpstreamModelName,
			ClientRequestedStream: info.ClientRequestedStream,
			RequestId:             info.RequestId,
			Group:                 info.UsingGroup,
			UpstreamURL:           upstreamReq.URL.String(),
			Payload:               string(payloadBytes),
		})
	}

	c.Set(channelRequestPreviewHandledKey, true)
	c.Abort()
	c.JSON(http.StatusOK, rawResp)
	return nil
}

func buildDownstreamPreviewBody(c *gin.Context) (previewBodyPayload, error) {
	storage, err := appcommon.GetBodyStorage(c)
	if err != nil {
		return previewBodyPayload{}, err
	}
	bodyBytes, err := storage.Bytes()
	if err != nil {
		return previewBodyPayload{}, err
	}
	return bodyPayloadFromBytes(bodyBytes, c.Request.Header.Get("Content-Type")), nil
}

func buildRequestBodyPayload(req *http.Request) (previewBodyPayload, error) {
	if req == nil || req.Body == nil {
		return previewBodyPayload{Kind: "empty"}, nil
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return previewBodyPayload{}, err
	}
	return bodyPayloadFromBytes(bodyBytes, req.Header.Get("Content-Type")), nil
}

func bodyPayloadFromBytes(data []byte, contentType string) previewBodyPayload {
	payload := previewBodyPayload{
		ContentType: strings.TrimSpace(contentType),
		Size:        int64(len(data)),
	}
	if len(data) == 0 {
		payload.Kind = "empty"
		return payload
	}

	normalizedType := normalizeMediaType(contentType)
	if strings.HasPrefix(normalizedType, "multipart/") {
		payload.Kind = "summary"
		payload.Summary = "Body omitted for multipart content."
		return payload
	}
	if isJSONMediaType(normalizedType) || looksLikeJSON(data) {
		var parsed any
		if err := appcommon.Unmarshal(data, &parsed); err == nil {
			payload.Kind = "json"
			payload.JSON = parsed
			return payload
		}
	}
	if isTextMediaType(normalizedType) {
		payload.Kind = "text"
		payload.Text = string(data)
		return payload
	}

	payload.Kind = "summary"
	payload.Summary = "Body omitted for binary or unsupported content."
	return payload
}

func canRevealSensitivePreviewData(c *gin.Context) bool {
	if c == nil {
		return false
	}
	role := c.GetInt("role")
	if role >= appcommon.RoleAdminUser {
		return true
	}
	userID := c.GetInt("id")
	if userID == 0 {
		return false
	}
	return model.IsAdmin(userID)
}

func writePreviewUnavailableResponse(c *gin.Context, info *relaycommon.RelayInfo) error {
	if c == nil {
		return fmt.Errorf("missing request context")
	}
	newAPIError := types.NewErrorWithStatusCode(
		fmt.Errorf("该模型正在调整，请稍后再试"),
		types.ErrorCodeBadResponse,
		http.StatusServiceUnavailable,
		types.ErrOptionWithSkipRetry(),
	)
	c.Set(channelRequestPreviewHandledKey, true)
	c.Abort()
	if info != nil && info.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		c.JSON(newAPIError.StatusCode, gin.H{
			"type":  "error",
			"error": newAPIError.ToClaudeError(),
		})
		return nil
	}
	c.JSON(newAPIError.StatusCode, gin.H{
		"error": newAPIError.ToOpenAIError(),
	})
	return nil
}

func shouldPreviewAsFormRequest(c *gin.Context, info *relaycommon.RelayInfo) bool {
	if c == nil || info == nil {
		return false
	}
	switch info.RelayMode {
	case relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		return true
	case relayconstant.RelayModeImagesEdits:
		return !isJSONMediaType(normalizeMediaType(c.Request.Header.Get("Content-Type")))
	default:
		return false
	}
}

func relayModeName(mode int) string {
	switch mode {
	case relayconstant.RelayModeChatCompletions:
		return "chat_completions"
	case relayconstant.RelayModeCompletions:
		return "completions"
	case relayconstant.RelayModeEmbeddings:
		return "embeddings"
	case relayconstant.RelayModeModerations:
		return "moderations"
	case relayconstant.RelayModeImagesGenerations:
		return "images_generations"
	case relayconstant.RelayModeImagesEdits:
		return "images_edits"
	case relayconstant.RelayModeAudioSpeech:
		return "audio_speech"
	case relayconstant.RelayModeAudioTranscription:
		return "audio_transcription"
	case relayconstant.RelayModeAudioTranslation:
		return "audio_translation"
	case relayconstant.RelayModeVideoSubmit:
		return "video_submit"
	case relayconstant.RelayModeRerank:
		return "rerank"
	case relayconstant.RelayModeResponses:
		return "responses"
	case relayconstant.RelayModeResponsesCompact:
		return "responses_compact"
	case relayconstant.RelayModeGemini:
		return "gemini"
	default:
		return "unknown"
	}
}

func relayFormatStrings(formats []types.RelayFormat) []string {
	if len(formats) == 0 {
		return nil
	}
	result := make([]string, 0, len(formats))
	for _, format := range formats {
		if format == "" {
			continue
		}
		result = append(result, string(format))
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneQueryValues(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string][]string, len(values))
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		next := make([]string, len(items))
		copy(next, items)
		cloned[key] = next
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func flattenHeader(header http.Header, host ...string) map[string]string {
	if len(header) == 0 && len(host) == 0 {
		return nil
	}
	flat := make(map[string]string, len(header)+1)
	for key := range header {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		flat[key] = value
	}
	if len(host) > 0 && strings.TrimSpace(host[0]) != "" {
		flat["Host"] = strings.TrimSpace(host[0])
	}
	if len(flat) == 0 {
		return nil
	}
	return flat
}

func maskHeaderMap(headers map[string]string, includeHost bool) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	masked := make(map[string]string, len(headers))
	for key, value := range headers {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if shouldMaskHeader(key) {
			masked[key] = maskHeaderValue(value)
			continue
		}
		if !includeHost && strings.EqualFold(key, "host") {
			continue
		}
		masked[key] = value
	}
	if len(masked) == 0 {
		return nil
	}
	return masked
}

func shouldMaskHeader(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch lower {
	case "authorization", "proxy-authorization", "x-api-key", "x-goog-api-key", "cookie", "set-cookie":
		return true
	}
	return strings.Contains(lower, "token") || strings.Contains(lower, "secret")
}

func maskHeaderValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) == 2 {
		return parts[0] + " ****"
	}
	return "****"
}

func normalizeMediaType(contentType string) string {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(trimmed, ";")[0]))
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func isJSONMediaType(mediaType string) bool {
	if mediaType == "" {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func isTextMediaType(mediaType string) bool {
	if mediaType == "" {
		return true
	}
	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/x-www-form-urlencoded", "application/xml", "application/javascript":
		return true
	default:
		return false
	}
}

func looksLikeJSON(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '{', '[', '"', 't', 'f', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}
