package hkcopp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type requestPayload struct {
	Model         string   `json:"model"`
	Mode          string   `json:"mode,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	Ratio         string   `json:"ratio,omitempty"`
	Duration      *int     `json:"duration,omitempty"`
	Resolution    string   `json:"resolution,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	ImageURLs     []string `json:"image_urls,omitempty"`
	VideoURLs     []string `json:"video_urls,omitempty"`
	AudioURLs     []string `json:"audio_urls,omitempty"`
	GenerateAudio *bool    `json:"generate_audio,omitempty"`
	WebSearch     *bool    `json:"web_search,omitempty"`
	Watermark     *bool    `json:"watermark,omitempty"`
	WebhookURL    string   `json:"webhook_url,omitempty"`
}

type metadataPayload struct {
	Ratio         string   `json:"ratio,omitempty"`
	Resolution    string   `json:"resolution,omitempty"`
	WebhookURL    string   `json:"webhook_url,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	ImageURLs     []string `json:"image_urls,omitempty"`
	VideoURLs     []string `json:"video_urls,omitempty"`
	AudioURLs     []string `json:"audio_urls,omitempty"`
	GenerateAudio *bool    `json:"generate_audio,omitempty"`
	WebSearch     *bool    `json:"web_search,omitempty"`
	Watermark     *bool    `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Mode        string   `json:"mode,omitempty"`
	Model       string   `json:"model,omitempty"`
	TaskType    string   `json:"task_type,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	Ratio       string   `json:"ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	ResultURL   string   `json:"result_url,omitempty"`
	ResultURLs  []string `json:"result_urls,omitempty"`
	Error       any      `json:"error,omitempty"`
	CreditsUsed int      `json:"credits_used,omitempty"`
	CreditsLeft int      `json:"credits_left,omitempty"`
	Duration    int      `json:"duration,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	CompletedAt string   `json:"completed_at,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "get_task_request_failed", http.StatusBadRequest)
	}

	switch deriveMode(req.Mode, req.Images) {
	case "text_to_video":
		info.Action = constant.TaskActionTextGenerate
	case "first_last_frame":
		info.Action = constant.TaskActionFirstTailGenerate
	case "multi_ref":
		info.Action = constant.TaskActionReferenceGenerate
	default:
		info.Action = constant.TaskActionGenerate
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return normalizeBaseURL(a.baseURL) + generationsEndpoint, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid task request type")
	}

	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	if resp == nil || resp.Body == nil {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("empty upstream response"), "empty_response", http.StatusBadGateway)
		return
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	var hResp responsePayload
	if err = common.Unmarshal(responseBody, &hResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if hResp.ID == "" {
		message := errorMessage(hResp.Error)
		if message == "" {
			message = "missing task id in HKCOPP response"
		}
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("%s", message), "task_failed", http.StatusBadRequest)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return hResp.ID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s%s/%s", normalizeBaseURL(baseURL), generationsEndpoint, url.PathEscape(taskID))
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	var meta metadataPayload
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &meta); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	images := normalizedImages(req, meta)
	mode := deriveMode(req.Mode, images)
	r := &requestPayload{
		Model:  taskcommon.DefaultString(info.UpstreamModelName, "seedance-2-official"),
		Mode:   mode,
		Prompt: req.Prompt,
		Ratio:  ratioFromSize(req.Size),
	}
	if duration, ok := requestDuration(req); ok {
		r.Duration = &duration
	}
	applyImages(r, mode, images)
	applyMetadata(r, meta)
	return r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var hResp responsePayload
	if err := common.Unmarshal(respBody, &hResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	taskInfo := &relaycommon.TaskInfo{
		TaskID: hResp.ID,
		Reason: errorMessage(hResp.Error),
	}
	switch normalizeStatus(hResp.Status) {
	case "created", "pending", "queued":
		taskInfo.Status = model.TaskStatusQueued
	case "processing", "running", "in_progress":
		taskInfo.Status = model.TaskStatusInProgress
	case "succeeded", "succeed", "success", "completed":
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Url = hResp.resultURL()
	case "failed", "error", "canceled", "cancelled":
		taskInfo.Status = model.TaskStatusFailure
		if taskInfo.Reason == "" {
			taskInfo.Reason = "task failed"
		}
	default:
		return nil, fmt.Errorf("unknown task status: %s", hResp.Status)
	}
	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var hResp responsePayload
	if len(originTask.Data) > 0 {
		if err := common.Unmarshal(originTask.Data, &hResp); err != nil {
			return nil, errors.Wrap(err, "unmarshal hkcopp task data failed")
		}
	}

	openAIVideo := originTask.ToOpenAIVideo()
	if openAIVideo.Model == "" {
		openAIVideo.Model = hResp.Model
	}
	if url := taskResultURL(originTask, hResp); url != "" {
		openAIVideo.SetMetadata("url", url)
	}
	if hResp.Duration > 0 {
		openAIVideo.Seconds = strconv.Itoa(hResp.Duration)
	}
	if hResp.Resolution != "" {
		openAIVideo.Size = hResp.Resolution
	} else if hResp.Ratio != "" {
		openAIVideo.Size = hResp.Ratio
	}
	if originTask.Status == model.TaskStatusFailure {
		message := originTask.FailReason
		if message == "" {
			message = errorMessage(hResp.Error)
		}
		if message == "" {
			message = "task failed"
		}
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: message,
			Code:    "task_failed",
		}
	}

	return common.Marshal(openAIVideo)
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return defaultBaseURL
	}
	if strings.HasSuffix(baseURL, "/openapi/v1") {
		return baseURL
	}
	return baseURL + "/openapi/v1"
}

func normalizedImages(req *relaycommon.TaskSubmitReq, meta metadataPayload) []string {
	images := append([]string(nil), req.Images...)
	if len(images) == 0 && strings.TrimSpace(req.Image) != "" {
		images = append(images, strings.TrimSpace(req.Image))
	}
	if len(images) == 0 && strings.TrimSpace(meta.ImageURL) != "" {
		images = append(images, strings.TrimSpace(meta.ImageURL))
	}
	if len(images) == 0 && len(meta.ImageURLs) > 0 {
		images = append(images, meta.ImageURLs...)
	}
	return images
}

func deriveMode(mode string, images []string) string {
	if strings.TrimSpace(mode) != "" {
		return strings.TrimSpace(mode)
	}
	switch len(images) {
	case 0:
		return "text_to_video"
	case 1:
		return "image_to_video"
	case 2:
		return "first_last_frame"
	default:
		return "multi_ref"
	}
}

func requestDuration(req *relaycommon.TaskSubmitReq) (int, bool) {
	if strings.TrimSpace(req.Seconds) != "" {
		seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
		if err == nil {
			return seconds, true
		}
	}
	if req.DurationSet {
		return req.Duration, true
	}
	if req.Duration > 0 {
		return req.Duration, true
	}
	return 0, false
}

func ratioFromSize(size string) string {
	switch strings.TrimSpace(size) {
	case "1280x720", "1920x1080":
		return "16:9"
	case "720x1280", "1080x1920":
		return "9:16"
	case "1024x1024", "512x512":
		return "1:1"
	default:
		return ""
	}
}

func applyImages(r *requestPayload, mode string, images []string) {
	if len(images) == 0 {
		return
	}
	switch mode {
	case "image_to_video":
		r.ImageURL = images[0]
	default:
		if len(images) == 1 {
			r.ImageURL = images[0]
			return
		}
		r.ImageURLs = images
	}
}

func applyMetadata(r *requestPayload, meta metadataPayload) {
	if meta.Ratio != "" {
		r.Ratio = meta.Ratio
	}
	if meta.Resolution != "" {
		r.Resolution = meta.Resolution
	}
	if meta.WebhookURL != "" {
		r.WebhookURL = meta.WebhookURL
	}
	if meta.ImageURL != "" {
		r.ImageURL = meta.ImageURL
	}
	if len(meta.ImageURLs) > 0 {
		r.ImageURLs = meta.ImageURLs
	}
	if len(meta.VideoURLs) > 0 {
		r.VideoURLs = meta.VideoURLs
	}
	if len(meta.AudioURLs) > 0 {
		r.AudioURLs = meta.AudioURLs
	}
	if meta.GenerateAudio != nil {
		r.GenerateAudio = meta.GenerateAudio
	}
	if meta.WebSearch != nil {
		r.WebSearch = meta.WebSearch
	}
	if meta.Watermark != nil {
		r.Watermark = meta.Watermark
	}
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func (r responsePayload) resultURL() string {
	if strings.TrimSpace(r.ResultURL) != "" {
		return strings.TrimSpace(r.ResultURL)
	}
	for _, resultURL := range r.ResultURLs {
		if strings.TrimSpace(resultURL) != "" {
			return strings.TrimSpace(resultURL)
		}
	}
	return ""
}

func taskResultURL(task *model.Task, hResp responsePayload) string {
	if url := strings.TrimSpace(task.GetResultURL()); url != "" {
		return url
	}
	return hResp.resultURL()
}

func errorMessage(v any) string {
	switch e := v.(type) {
	case nil:
		return ""
	case string:
		return e
	case map[string]any:
		if message, ok := e["message"].(string); ok && message != "" {
			return message
		}
		if code, ok := e["code"].(string); ok && code != "" {
			return code
		}
	case map[string]string:
		if e["message"] != "" {
			return e["message"]
		}
		if e["code"] != "" {
			return e["code"]
		}
	}
	bytes, err := common.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(bytes)
}
