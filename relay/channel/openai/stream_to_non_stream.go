package openai

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const streamToNonStreamMaxBufferSize = 64 << 20

type aggregatedChatCompletionResponse struct {
	Id                string                         `json:"id"`
	Object            string                         `json:"object"`
	Created           int64                          `json:"created"`
	Model             string                         `json:"model"`
	SystemFingerprint *string                        `json:"system_fingerprint,omitempty"`
	Choices           []dto.OpenAITextResponseChoice `json:"choices"`
	Usage             dto.Usage                      `json:"usage"`
}

type chatStreamChoiceAccumulator struct {
	index           int
	role            string
	content         strings.Builder
	reasoning       strings.Builder
	finishReason    string
	hasFinishReason bool
	toolCalls       map[int]*dto.ToolCallResponse
	toolOrder       []int
}

type chatStreamAccumulator struct {
	id                string
	created           int64
	model             string
	systemFingerprint *string
	choices           map[int]*chatStreamChoiceAccumulator
	usage             *dto.Usage
	lastStreamData    string
}

func OaiStreamToOpenAIHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	response, usage, err := collectOpenAIStreamAsChatCompletion(c, info, resp)
	if err != nil {
		return nil, err
	}

	responseBody, marshalErr := common.Marshal(response)
	if marshalErr != nil {
		return nil, types.NewError(marshalErr, types.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, nil, responseBody)
	return usage, nil
}

func collectOpenAIStreamAsChatCompletion(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*aggregatedChatCompletionResponse, *dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	if info.RelayMode != relayconstant.RelayModeChatCompletions {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("upstream stream to non-stream is only supported for chat completions"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	accumulator := &chatStreamAccumulator{
		model:   info.UpstreamModelName,
		choices: make(map[int]*chatStreamChoiceAccumulator),
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), streamToNonStreamMaxBufferSize)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 6 {
			continue
		}
		if !strings.HasPrefix(line, "data:") && !strings.HasPrefix(line, "[DONE]") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if strings.HasPrefix(data, "[DONE]") {
			break
		}
		info.SetFirstResponseTime()
		info.ReceivedResponseCount++
		accumulator.lastStreamData = data

		if apiErr := aggregateOpenAIStreamData(accumulator, data, resp.StatusCode); apiErr != nil {
			return nil, nil, apiErr
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	response, usage := accumulator.toResponse(c, info)
	if response == nil {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("empty upstream stream response"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	return response, usage, nil
}

func aggregateOpenAIStreamData(accumulator *chatStreamAccumulator, data string, statusCode int) *types.NewAPIError {
	var simple dto.SimpleResponse
	if err := common.UnmarshalJsonStr(data, &simple); err == nil {
		if oaiError := simple.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
			return types.WithOpenAIError(*oaiError, statusCode)
		}
	}

	var streamResponse dto.ChatCompletionsStreamResponse
	if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if streamResponse.Id != "" {
		accumulator.id = streamResponse.Id
	}
	if streamResponse.Created != 0 {
		accumulator.created = streamResponse.Created
	}
	if streamResponse.Model != "" {
		accumulator.model = streamResponse.Model
	}
	if streamResponse.SystemFingerprint != nil {
		accumulator.systemFingerprint = streamResponse.SystemFingerprint
	}
	if service.ValidUsage(streamResponse.Usage) {
		accumulator.usage = streamResponse.Usage
	}

	for _, streamChoice := range streamResponse.Choices {
		choice := accumulator.choice(streamChoice.Index)
		if streamChoice.Delta.Role != "" {
			choice.role = streamChoice.Delta.Role
		}
		choice.content.WriteString(streamChoice.Delta.GetContentString())
		choice.reasoning.WriteString(streamChoice.Delta.GetReasoningContent())
		if streamChoice.FinishReason != nil {
			choice.finishReason = *streamChoice.FinishReason
			choice.hasFinishReason = true
		}
		for _, toolCall := range streamChoice.Delta.ToolCalls {
			choice.mergeToolCall(toolCall)
		}
	}

	return nil
}

func (a *chatStreamAccumulator) choice(index int) *chatStreamChoiceAccumulator {
	if existing, ok := a.choices[index]; ok {
		return existing
	}
	choice := &chatStreamChoiceAccumulator{
		index:     index,
		toolCalls: make(map[int]*dto.ToolCallResponse),
	}
	a.choices[index] = choice
	return choice
}

func (c *chatStreamChoiceAccumulator) mergeToolCall(toolCall dto.ToolCallResponse) {
	index := 0
	if toolCall.Index != nil {
		index = *toolCall.Index
	}

	existing, ok := c.toolCalls[index]
	if !ok {
		c.toolOrder = append(c.toolOrder, index)
		existing = &dto.ToolCallResponse{}
		c.toolCalls[index] = existing
	}

	if toolCall.ID != "" {
		existing.ID = toolCall.ID
	}
	if toolCall.Type != nil {
		existing.Type = toolCall.Type
	}
	if toolCall.Function.Name != "" {
		existing.Function.Name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		existing.Function.Arguments += toolCall.Function.Arguments
	}
}

func (a *chatStreamAccumulator) toResponse(c *gin.Context, info *relaycommon.RelayInfo) (*aggregatedChatCompletionResponse, *dto.Usage) {
	if len(a.choices) == 0 {
		return nil, nil
	}

	indices := make([]int, 0, len(a.choices))
	for index := range a.choices {
		indices = append(indices, index)
	}
	sort.Ints(indices)

	choices := make([]dto.OpenAITextResponseChoice, 0, len(indices))
	for _, index := range indices {
		choice := a.choices[index]
		message := dto.Message{
			Role: choice.roleOrDefault(),
		}
		content := choice.content.String()
		toolCalls := choice.finalToolCalls()
		if len(toolCalls) == 0 || content != "" {
			message.SetStringContent(content)
		} else {
			message.SetNullContent()
		}
		if reasoning := choice.reasoning.String(); reasoning != "" {
			message.ReasoningContent = &reasoning
		}
		if len(toolCalls) > 0 {
			message.SetToolCalls(toolCalls)
		}

		choices = append(choices, dto.OpenAITextResponseChoice{
			Index:        index,
			Message:      message,
			FinishReason: choice.finishReasonOrDefault(),
		})
	}

	usage := a.usage
	if !service.ValidUsage(usage) {
		usage = service.ResponseText2Usage(c, a.usageText(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		toolCount := a.toolCount()
		if toolCount > 0 {
			usage.CompletionTokens += toolCount * 7
			usage.TotalTokens += toolCount * 7
		}
	}
	applyUsagePostProcessing(info, usage, common.StringToByteSlice(a.lastStreamData))

	id := a.id
	if id == "" {
		id = helper.GetResponseID(c)
	}
	created := a.created
	if created == 0 {
		created = time.Now().Unix()
	}
	model := a.model
	if model == "" {
		model = info.UpstreamModelName
	}

	return &aggregatedChatCompletionResponse{
		Id:                id,
		Object:            "chat.completion",
		Created:           created,
		Model:             model,
		SystemFingerprint: a.systemFingerprint,
		Choices:           choices,
		Usage:             *usage,
	}, usage
}

func (c *chatStreamChoiceAccumulator) roleOrDefault() string {
	if c.role != "" {
		return c.role
	}
	return "assistant"
}

func (c *chatStreamChoiceAccumulator) finishReasonOrDefault() string {
	if c.hasFinishReason {
		return c.finishReason
	}
	return "stop"
}

func (c *chatStreamChoiceAccumulator) finalToolCalls() []dto.ToolCallResponse {
	if len(c.toolCalls) == 0 {
		return nil
	}
	sort.Ints(c.toolOrder)
	toolCalls := make([]dto.ToolCallResponse, 0, len(c.toolOrder))
	for _, index := range c.toolOrder {
		toolCall := *c.toolCalls[index]
		toolCall.Index = nil
		toolCalls = append(toolCalls, toolCall)
	}
	return toolCalls
}

func (a *chatStreamAccumulator) usageText() string {
	var builder strings.Builder
	indices := make([]int, 0, len(a.choices))
	for index := range a.choices {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		choice := a.choices[index]
		builder.WriteString(choice.content.String())
		builder.WriteString(choice.reasoning.String())
		for _, toolCall := range choice.finalToolCalls() {
			builder.WriteString(toolCall.Function.Name)
			builder.WriteString(toolCall.Function.Arguments)
		}
	}
	return builder.String()
}

func (a *chatStreamAccumulator) toolCount() int {
	count := 0
	for _, choice := range a.choices {
		count += len(choice.toolCalls)
	}
	return count
}
