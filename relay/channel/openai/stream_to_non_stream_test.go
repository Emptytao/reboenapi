package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestCollectOpenAIStreamAsChatCompletionAggregatesContentAndUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	body := strings.Join([]string{
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"gpt-test","system_fingerprint":"fp_test","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"gpt-test","choices":[{"index":0,"delta":{"reasoning_content":"think "},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hel"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":123,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`,
		`data: [DONE]`,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	response, usage, apiErr := collectOpenAIStreamAsChatCompletion(c, info, resp)
	if apiErr != nil {
		t.Fatalf("collectOpenAIStreamAsChatCompletion returned error: %v", apiErr)
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 2 || usage.TotalTokens != 7 {
		t.Fatalf("usage = %+v, want prompt=5 completion=2 total=7", usage)
	}
	if response.Id != "chatcmpl-1" || response.Object != "chat.completion" || response.Created != 123 || response.Model != "gpt-test" {
		t.Fatalf("unexpected response metadata: %+v", response)
	}
	if response.SystemFingerprint == nil || *response.SystemFingerprint != "fp_test" {
		t.Fatalf("system fingerprint = %v, want fp_test", response.SystemFingerprint)
	}
	if len(response.Choices) != 1 {
		t.Fatalf("choices len = %d, want 1", len(response.Choices))
	}
	choice := response.Choices[0]
	if choice.Message.Role != "assistant" {
		t.Fatalf("role = %q, want assistant", choice.Message.Role)
	}
	if choice.Message.StringContent() != "hello" {
		t.Fatalf("content = %q, want hello", choice.Message.StringContent())
	}
	if choice.Message.GetReasoningContent() != "think " {
		t.Fatalf("reasoning = %q, want think ", choice.Message.GetReasoningContent())
	}
	if choice.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q, want stop", choice.FinishReason)
	}
}

func TestCollectOpenAIStreamAsChatCompletionAggregatesToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	toolIndex := 0
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
	}
	accumulator := &chatStreamAccumulator{
		model:   "gpt-test",
		choices: make(map[int]*chatStreamChoiceAccumulator),
	}

	firstChunk := dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tools",
		Created: 456,
		Model:   "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Role: "assistant",
				ToolCalls: []dto.ToolCallResponse{{
					Index: &toolIndex,
					ID:    "call_1",
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "lookup",
						Arguments: `{"city":"`,
					},
				}},
			},
		}},
	}
	secondChunk := dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl-tools",
		Created: 456,
		Model:   "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: &toolIndex,
					Function: dto.FunctionResponse{
						Arguments: `Paris"}`,
					},
				}},
			},
			FinishReason: common.GetPointer("tool_calls"),
		}},
		Usage: &dto.Usage{PromptTokens: 4, CompletionTokens: 3, TotalTokens: 7},
	}

	firstData, err := common.Marshal(firstChunk)
	if err != nil {
		t.Fatalf("marshal first chunk: %v", err)
	}
	secondData, err := common.Marshal(secondChunk)
	if err != nil {
		t.Fatalf("marshal second chunk: %v", err)
	}
	if apiErr := aggregateOpenAIStreamData(accumulator, string(firstData), http.StatusOK); apiErr != nil {
		t.Fatalf("aggregate first chunk: %v", apiErr)
	}
	if apiErr := aggregateOpenAIStreamData(accumulator, string(secondData), http.StatusOK); apiErr != nil {
		t.Fatalf("aggregate second chunk: %v", apiErr)
	}

	response, usage := accumulator.toResponse(c, info)
	if usage.TotalTokens != 7 {
		t.Fatalf("usage total = %d, want 7", usage.TotalTokens)
	}
	toolCalls := response.Choices[0].Message.ParseToolCalls()
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls len = %d, want 1", len(toolCalls))
	}
	if toolCalls[0].ID != "call_1" || toolCalls[0].Function.Name != "lookup" || toolCalls[0].Function.Arguments != `{"city":"Paris"}` {
		t.Fatalf("tool call = %+v", toolCalls[0])
	}
	if response.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish_reason = %q, want tool_calls", response.Choices[0].FinishReason)
	}
}

func TestOaiStreamToOpenAIHandlerWritesNonStreamJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:             relayconstant.RelayModeChatCompletions,
		ChannelMeta:           &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:              true,
		UpstreamIsStream:      true,
		ClientRequestedStream: false,
	}
	body := strings.Join([]string{
		`data: {"id":"chatcmpl-json","created":789,"model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		`data: [DONE]`,
	}, "\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, apiErr := OaiStreamToOpenAIHandler(c, info, resp)
	if apiErr != nil {
		t.Fatalf("OaiStreamToOpenAIHandler returned error: %v", apiErr)
	}
	if usage.TotalTokens != 2 {
		t.Fatalf("usage total = %d, want 2", usage.TotalTokens)
	}
	if strings.Contains(recorder.Body.String(), "data:") {
		t.Fatalf("handler wrote SSE data instead of JSON: %s", recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	var response aggregatedChatCompletionResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response body: %v", err)
	}
	if response.Choices[0].Message.StringContent() != "done" {
		t.Fatalf("content = %q, want done", response.Choices[0].Message.StringContent())
	}
}
