package channel

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

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
	RawHTTP string `json:"raw_http"`
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
	downstreamPacket, err := buildDownstreamPreviewPacket(c)
	if err != nil {
		return err
	}
	upstreamPacket, err := buildUpstreamPreviewPacket(upstreamReq)
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
		Downstream:      downstreamPacket,
		UpstreamRequest: upstreamPacket,
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
	return writePreviewUnavailableResponse(c, info)
}

func buildDownstreamPreviewPacket(c *gin.Context) (previewRequestPayload, error) {
	storage, err := appcommon.GetBodyStorage(c)
	if err != nil {
		return previewRequestPayload{}, err
	}
	bodyBytes, err := storage.Bytes()
	if err != nil {
		return previewRequestPayload{}, err
	}
	return previewRequestPayload{
		RawHTTP: buildHTTPRequestMessage(c.Request, bodyBytes),
	}, nil
}

func buildUpstreamPreviewPacket(req *http.Request) (previewRequestPayload, error) {
	if req == nil || req.Body == nil {
		return previewRequestPayload{RawHTTP: buildHTTPRequestMessage(req, nil)}, nil
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return previewRequestPayload{}, err
	}
	return previewRequestPayload{
		RawHTTP: buildHTTPRequestMessage(req, bodyBytes),
	}, nil
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

func buildHTTPRequestMessage(req *http.Request, bodyBytes []byte) string {
	if req == nil {
		return ""
	}
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = http.MethodPost
	}
	target := "/"
	if req.URL != nil {
		if requestURI := strings.TrimSpace(req.URL.RequestURI()); requestURI != "" {
			target = requestURI
		}
	}
	proto := strings.TrimSpace(req.Proto)
	if proto == "" {
		proto = "HTTP/1.1"
	}

	var builder strings.Builder
	builder.WriteString(method)
	builder.WriteString(" ")
	builder.WriteString(target)
	builder.WriteString(" ")
	builder.WriteString(proto)
	builder.WriteString("\r\n")

	if host := strings.TrimSpace(req.Host); host != "" {
		builder.WriteString("Host: ")
		builder.WriteString(host)
		builder.WriteString("\r\n")
	}
	if req.ContentLength >= 0 && req.Header.Get("Content-Length") == "" {
		builder.WriteString("Content-Length: ")
		builder.WriteString(fmt.Sprintf("%d", req.ContentLength))
		builder.WriteString("\r\n")
	}

	keys := make([]string, 0, len(req.Header))
	for key := range req.Header {
		if strings.EqualFold(key, "host") || strings.EqualFold(key, "content-length") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := req.Header.Values(key)
		if len(values) == 0 {
			continue
		}
		for _, value := range values {
			builder.WriteString(key)
			builder.WriteString(": ")
			builder.WriteString(strings.TrimSpace(value))
			builder.WriteString("\r\n")
		}
	}
	builder.WriteString("\r\n")
	builder.WriteString(bodyTextFromBytes(bodyBytes, req.Header.Get("Content-Type")))
	return builder.String()
}

func bodyTextFromBytes(data []byte, contentType string) string {
	if len(data) == 0 {
		return ""
	}
	normalizedType := normalizeMediaType(contentType)
	switch {
	case strings.HasPrefix(normalizedType, "multipart/"):
		if rendered, ok := buildMultipartPreviewBody(data, contentType); ok {
			return rendered
		}
		return fmt.Sprintf("[multipart body omitted in preview, %d bytes]", len(data))
	case isJSONMediaType(normalizedType):
		return string(data)
	case looksLikeJSON(data):
		return string(data)
	case isTextMediaType(normalizedType):
		return string(data)
	case normalizedType == "" && utf8.Valid(data):
		return string(data)
	default:
		return fmt.Sprintf("[binary or unsupported body omitted in preview, content-type=%q, %d bytes]", strings.TrimSpace(contentType), len(data))
	}
}

func buildMultipartPreviewBody(data []byte, contentType string) (string, bool) {
	_, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return "", false
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return "", false
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)
	var builder strings.Builder
	partCount := 0

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}

		partCount++
		builder.WriteString("--")
		builder.WriteString(boundary)
		builder.WriteString("\r\n")

		keys := make([]string, 0, len(part.Header))
		for key := range part.Header {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			values := part.Header.Values(key)
			for _, value := range values {
				builder.WriteString(key)
				builder.WriteString(": ")
				builder.WriteString(strings.TrimSpace(value))
				builder.WriteString("\r\n")
			}
		}
		builder.WriteString("\r\n")

		partBytes, err := io.ReadAll(part)
		if err != nil {
			return "", false
		}
		if shouldInlineMultipartPart(part, partBytes) {
			builder.Write(partBytes)
		} else {
			builder.WriteString(buildMultipartPartPlaceholder(part, partBytes))
		}
		builder.WriteString("\r\n")
	}

	if partCount == 0 {
		return "", false
	}

	builder.WriteString("--")
	builder.WriteString(boundary)
	builder.WriteString("--")
	return builder.String(), true
}

func shouldInlineMultipartPart(part *multipart.Part, data []byte) bool {
	if part == nil {
		return false
	}
	if part.FileName() != "" {
		return false
	}
	normalizedType := normalizeMediaType(part.Header.Get("Content-Type"))
	if isJSONMediaType(normalizedType) || isTextMediaType(normalizedType) {
		return true
	}
	return utf8.Valid(data)
}

func buildMultipartPartPlaceholder(part *multipart.Part, data []byte) string {
	if part == nil {
		return fmt.Sprintf("[multipart content omitted in preview, %d bytes]", len(data))
	}
	name := strings.TrimSpace(part.FormName())
	filename := strings.TrimSpace(part.FileName())
	contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	var segments []string
	if name != "" {
		segments = append(segments, fmt.Sprintf("name=%q", name))
	}
	if filename != "" {
		segments = append(segments, fmt.Sprintf("filename=%q", filename))
	}
	segments = append(segments, fmt.Sprintf("content_type=%q", contentType))
	segments = append(segments, fmt.Sprintf("size=%d bytes", len(data)))
	return "[multipart file content omitted in preview, " + strings.Join(segments, ", ") + "]"
}

func writePreviewUnavailableResponse(c *gin.Context, info *relaycommon.RelayInfo) error {
	if c == nil {
		return fmt.Errorf("missing request context")
	}
	newAPIError := types.NewErrorWithStatusCode(
		fmt.Errorf("该模型正在调试中"),
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
