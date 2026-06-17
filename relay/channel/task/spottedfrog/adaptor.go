package spottedfrog

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path"
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

const spottedFrogRequestContentTypeKey = "spottedfrog_request_content_type"

type requestPayload struct {
	Model           string         `json:"model"`
	Prompt          string         `json:"prompt,omitempty"`
	Size            string         `json:"size,omitempty"`
	Seconds         any            `json:"seconds,omitempty"`
	Duration        any            `json:"duration,omitempty"`
	AspectRatio     string         `json:"aspect_ratio,omitempty"`
	Image           string         `json:"image,omitempty"`
	Images          []string       `json:"images,omitempty"`
	InputReference  any            `json:"input_reference,omitempty"`
	ReferenceImages []string       `json:"reference_images,omitempty"`
	ReferenceMode   string         `json:"reference_mode,omitempty"`
	GenerateAudio   *bool          `json:"generate_audio,omitempty"`
	Fps             *int           `json:"fps,omitempty"`
	Seed            *int           `json:"seed,omitempty"`
	N               *int           `json:"n,omitempty"`
	ResponseFormat  string         `json:"response_format,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type metadataPayload struct {
	Variant         string   `json:"variant,omitempty"`
	Speed           string   `json:"speed,omitempty"`
	Resolution      string   `json:"resolution,omitempty"`
	ResolutionName  string   `json:"resolution_name,omitempty"`
	AspectRatio     string   `json:"aspect_ratio,omitempty"`
	Ratio           string   `json:"ratio,omitempty"`
	Audio           *bool    `json:"audio,omitempty"`
	ReferenceMode   string   `json:"reference_mode,omitempty"`
	ReferenceImages []string `json:"reference_images,omitempty"`
	Image           string   `json:"image,omitempty"`
	Images          []string `json:"images,omitempty"`
	GenerateAudio   *bool    `json:"generate_audio,omitempty"`
	WebhookURL      string   `json:"webhook_url,omitempty"`
}

type responsePayload struct {
	ID          string   `json:"id"`
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	URL         string   `json:"url,omitempty"`
	VideoURL    string   `json:"video_url,omitempty"`
	ResultURL   string   `json:"result_url,omitempty"`
	OutputURL   string   `json:"output_url,omitempty"`
	URLs        []string `json:"urls,omitempty"`
	ResultURLs  []string `json:"result_urls,omitempty"`
	Progress    any      `json:"progress,omitempty"`
	Error       any      `json:"error,omitempty"`
	Seconds     any      `json:"seconds,omitempty"`
	Duration    any      `json:"duration,omitempty"`
	Size        string   `json:"size,omitempty"`
	CreatedAt   int64    `json:"created_at,omitempty"`
	CompletedAt int64    `json:"completed_at,omitempty"`
}

type omniJSONRequestPayload struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt,omitempty"`
	Size           string `json:"size,omitempty"`
	Seconds        string `json:"seconds,omitempty"`
	InputReference string `json:"input_reference,omitempty"`
	GenerateAudio  *bool  `json:"generate_audio,omitempty"`
}

type omniInputFile struct {
	Filename    string
	ContentType string
	Data        []byte
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
	var meta metadataPayload
	_ = taskcommon.UnmarshalMetadata(req.Metadata, &meta)
	imagesCount := len(normalizedImages(&req, meta))
	if imagesCount == 0 {
		imagesCount = countMultipartReferenceFiles(c)
	}
	if isGrokRequest(&req, info) {
		imagesCount = grokReferenceCount(c, &req, meta)
		if imagesCount == 0 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("grok-imagine-video requires at least one reference image"), "invalid_request", http.StatusBadRequest)
		}
	}
	if isOmniFlashRequest(&req, info) && imagesCount > 7 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("omni_flash supports at most 7 reference images"), "invalid_request", http.StatusBadRequest)
	}
	switch {
	case imagesCount == 0:
		info.Action = constant.TaskActionTextGenerate
	case imagesCount == 2:
		info.Action = constant.TaskActionFirstTailGenerate
	case imagesCount > 2:
		info.Action = constant.TaskActionReferenceGenerate
	default:
		info.Action = constant.TaskActionGenerate
	}
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if isGrokRequest(nil, info) {
		return normalizeBaseURL(a.baseURL) + "/v1" + videosEndpoint, nil
	}
	return normalizeBaseURL(a.baseURL) + "/v1" + videosEndpoint + "?async=true", nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	contentType := "application/json"
	if c != nil {
		if value, exists := c.Get(spottedFrogRequestContentTypeKey); exists {
			if contentTypeStr, ok := value.(string); ok && strings.TrimSpace(contentTypeStr) != "" {
				contentType = strings.TrimSpace(contentTypeStr)
			}
		}
	}
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	if !isGrokRequest(nil, info) {
		req.Header.Set("Prefer", "respond-async")
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	if isOmniFlashRequest(&req, info) {
		return a.buildOmniRequestBody(c, &req, info)
	}
	if isGrokRequest(&req, info) {
		return a.buildGrokRequestBody(c, &req, info)
	}
	body, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	setSpottedFrogRequestContentType(c, "application/json")
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
	var sfResp responsePayload
	if err = common.Unmarshal(responseBody, &sfResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	upstreamID := sfResp.upstreamID()
	if upstreamID == "" {
		message := errorMessage(sfResp.Error)
		if message == "" {
			message = "missing task id in SpottedFrog response"
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
	return upstreamID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	if modelName, _ := body["origin_model"].(string); isGrokModelName(modelName) {
		uri := fmt.Sprintf("%s/v1%s/%s", normalizeBaseURL(baseURL), videosEndpoint, url.PathEscape(taskID))
		return doSpottedFrogFetchTask(uri, key, proxy)
	}
	if modelName, _ := body["model"].(string); isGrokModelName(modelName) {
		uri := fmt.Sprintf("%s/v1%s/%s", normalizeBaseURL(baseURL), videosEndpoint, url.PathEscape(taskID))
		return doSpottedFrogFetchTask(uri, key, proxy)
	}
	asyncURI := fmt.Sprintf("%s/v1%s/%s?async=true", normalizeBaseURL(baseURL), videosEndpoint, url.PathEscape(taskID))
	resp, err := doSpottedFrogFetchTask(asyncURI, key, proxy)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.StatusCode < http.StatusBadRequest {
		return resp, nil
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	uri := fmt.Sprintf("%s/v1%s/%s", normalizeBaseURL(baseURL), videosEndpoint, url.PathEscape(taskID))
	return doSpottedFrogFetchTask(uri, key, proxy)
}

func doSpottedFrogFetchTask(uri, key, proxy string) (*http.Response, error) {
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
	duration, durationSet := requestDuration(req)
	aspect := requestAspectRatio(req.Size, meta)
	body := &requestPayload{
		Model:          taskcommon.DefaultString(info.UpstreamModelName, req.Model),
		Prompt:         req.Prompt,
		Size:           req.Size,
		AspectRatio:    aspect,
		Image:          req.Image,
		Images:         req.Images,
		Fps:            req.Fps,
		Seed:           req.Seed,
		N:              req.N,
		ResponseFormat: req.ResponseFormat,
		Metadata:       sanitizedMetadata(req.Metadata),
	}
	if durationSet {
		body.Duration = duration
		body.Seconds = duration
	}
	useReferenceImages := strings.Contains(info.UpstreamModelName, "veo31-ref") || len(meta.ReferenceImages) > 0
	if len(meta.ReferenceImages) > 0 {
		images = meta.ReferenceImages
	}
	if useReferenceImages && len(images) > 0 {
		body.ReferenceImages = images
		body.ReferenceMode = taskcommon.DefaultString(meta.ReferenceMode, "image")
		body.Image = ""
		body.Images = nil
	} else if len(images) == 1 {
		body.Image = images[0]
		body.Images = nil
	} else if len(images) > 1 {
		body.ReferenceImages = images
		body.ReferenceMode = taskcommon.DefaultString(meta.ReferenceMode, "image")
		body.Images = nil
	}
	if meta.GenerateAudio != nil {
		body.GenerateAudio = meta.GenerateAudio
	}
	if req.InputReference != "" {
		body.InputReference = req.InputReference
	}
	return body, nil
}

func (a *TaskAdaptor) buildOmniRequestBody(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (io.Reader, error) {
	var meta metadataPayload
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &meta); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}
	files, err := collectOmniInputFiles(c, req, meta)
	if err != nil {
		return nil, err
	}
	if len(files) > 7 {
		return nil, fmt.Errorf("omni_flash supports at most 7 reference images")
	}

	modelName := taskcommon.DefaultString(info.UpstreamModelName, req.Model)
	if strings.EqualFold(strings.TrimSpace(modelName), "omni") {
		modelName = "omni_flash"
	}
	size := omniSize(req.Size, meta)
	seconds := omniSeconds(req)
	generateAudio := omniGenerateAudio(meta)

	if len(files) == 0 {
		body := omniJSONRequestPayload{
			Model:          modelName,
			Prompt:         req.Prompt,
			Size:           size,
			Seconds:        seconds,
			InputReference: "[]",
		}
		if generateAudio {
			body.GenerateAudio = boolPtr(true)
		}
		data, err := common.Marshal(body)
		if err != nil {
			return nil, err
		}
		setSpottedFrogRequestContentType(c, "application/json")
		return bytes.NewReader(data), nil
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("model", modelName); err != nil {
		return nil, err
	}
	if err := writer.WriteField("prompt", req.Prompt); err != nil {
		return nil, err
	}
	if err := writer.WriteField("size", size); err != nil {
		return nil, err
	}
	if err := writer.WriteField("seconds", seconds); err != nil {
		return nil, err
	}
	if generateAudio {
		if err := writer.WriteField("generate_audio", "true"); err != nil {
			return nil, err
		}
	}
	for idx, file := range files {
		if err := writeOmniMultipartFile(writer, idx, file); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	setSpottedFrogRequestContentType(c, writer.FormDataContentType())
	return &buf, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var sfResp responsePayload
	if err := common.Unmarshal(respBody, &sfResp); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}
	taskInfo := &relaycommon.TaskInfo{
		TaskID: sfResp.upstreamID(),
		Reason: errorMessage(sfResp.Error),
	}
	if progress := progressString(sfResp.Progress); progress != "" {
		taskInfo.Progress = progress
	}
	switch normalizeStatus(sfResp.Status) {
	case "created", "pending", "queued":
		taskInfo.Status = model.TaskStatusQueued
	case "processing", "running", "in_progress":
		taskInfo.Status = model.TaskStatusInProgress
	case "succeeded", "succeed", "success", "completed":
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Url = sfResp.resultURL()
	case "failed", "error", "canceled", "cancelled":
		taskInfo.Status = model.TaskStatusFailure
		if taskInfo.Reason == "" {
			taskInfo.Reason = "task failed"
		}
	default:
		return nil, fmt.Errorf("unknown task status: %s", sfResp.Status)
	}
	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var sfResp responsePayload
	if len(originTask.Data) > 0 {
		_ = common.Unmarshal(originTask.Data, &sfResp)
	}
	openAIVideo := originTask.ToOpenAIVideo()
	if url := taskResultURL(originTask, sfResp); url != "" {
		openAIVideo.SetMetadata("url", url)
	}
	if seconds := secondsString(sfResp); seconds != "" {
		openAIVideo.Seconds = seconds
	}
	if sfResp.Size != "" {
		openAIVideo.Size = sfResp.Size
	}
	if originTask.Status == model.TaskStatusFailure {
		message := originTask.FailReason
		if message == "" {
			message = errorMessage(sfResp.Error)
		}
		if message == "" {
			message = "task failed"
		}
		openAIVideo.Error = &dto.OpenAIVideoError{Message: message, Code: "task_failed"}
	}
	return common.Marshal(openAIVideo)
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return defaultBaseURL
	}
	return baseURL
}

func applyModelMapping(_ *gin.Context, info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) error {
	if info == nil {
		return fmt.Errorf("relay info is nil")
	}
	if info.UpstreamModelName == "" {
		info.UpstreamModelName = req.Model
	}
	upstream, err := mapUpstreamModel(req, info.UpstreamModelName, info.ChannelOtherSettings.SpottedFrogModelMap)
	if err != nil {
		return err
	}
	info.UpstreamModelName = upstream
	return nil
}

func mapUpstreamModel(req *relaycommon.TaskSubmitReq, modelName string, overrides *dto.SpottedFrogModelMap) (string, error) {
	logicalModel := strings.TrimSpace(modelName)
	if logicalModel == "" {
		logicalModel = strings.TrimSpace(req.Model)
	}
	effectiveModelMap := mergeSpottedFrogModelMap(overrides)
	var meta metadataPayload
	_ = taskcommon.UnmarshalMetadata(req.Metadata, &meta)
	duration, ok := requestDuration(req)
	aspect := aspectToken(requestAspectRatio(req.Size, meta))
	switch normalizeVideoModelName(logicalModel) {
	case "sora-2":
		if !ok || duration <= 0 {
			duration = 12
		}
		if duration != 4 && duration != 8 && duration != 12 {
			return "", fmt.Errorf("sora-2 duration must be one of 4, 8, 12")
		}
		if aspect == "" {
			aspect = "16x9"
		}
		if aspect != "16x9" && aspect != "9x16" {
			return "", fmt.Errorf("sora-2 aspect ratio must be 16:9 or 9:16")
		}
		if strings.EqualFold(meta.Variant, "pro") {
			if duration != 12 {
				return "", fmt.Errorf("sora-2 pro only supports 12 seconds")
			}
			return mapSoraProModel(effectiveModelMap, aspect), nil
		}
		return mapSoraModel(effectiveModelMap, duration, aspect), nil
	case "omni", "omni_flash", "grok", "grok-imagine-video", "grok-imagine-video-1.5-preview":
		if normalizeVideoModelName(logicalModel) == "omni_flash" || normalizeVideoModelName(logicalModel) == "omni" {
			if len(normalizedImages(req, meta)) > 7 {
				return "", fmt.Errorf("omni_flash supports at most 7 reference images")
			}
			return effectiveModelMap.OmniFlash, nil
		}
		return effectiveModelMap.GrokImagineVideo, nil
	case "veo":
		if !ok || duration <= 0 {
			duration = 8
		}
		if aspect == "" {
			aspect = "16x9"
		}
		resolution := resolutionToken(meta.Resolution, req.Size)
		if resolution == "" {
			resolution = "1080p"
		}
		mode := "fast"
		if strings.EqualFold(meta.Speed, "standard") {
			mode = "standard"
		}
		hasReference := len(normalizedImages(req, meta)) > 0
		return mapVeoModel(effectiveModelMap, mode, duration, aspect, resolution, hasReference), nil
	default:
		return logicalModel, nil
	}
}

func mapSoraModel(effective dto.SpottedFrogModelMap, duration int, aspect string) string {
	switch aspect {
	case "16x9":
		switch duration {
		case 4:
			return effective.Sora216x94s
		case 8:
			return effective.Sora216x98s
		default:
			return effective.Sora216x912s
		}
	case "9x16":
		switch duration {
		case 4:
			return effective.Sora29x164s
		case 8:
			return effective.Sora29x168s
		default:
			return effective.Sora29x1612s
		}
	default:
		return fmt.Sprintf("sora-2-%ds-%s", duration, aspect)
	}
}

func mapSoraProModel(effective dto.SpottedFrogModelMap, aspect string) string {
	switch aspect {
	case "9x16":
		return effective.Sora2Pro9x1612s
	default:
		return effective.Sora2Pro16x912s
	}
}

func mapVeoModel(effective dto.SpottedFrogModelMap, mode string, duration int, aspect, resolution string, hasReference bool) string {
	if hasReference {
		mode = "ref"
	}
	if modelName, ok := fixedVeoModel(effective, mode, duration, aspect, resolution); ok {
		return modelName
	}
	if hasReference {
		return fmt.Sprintf("firefly-veo31-ref-%ds-%s-%s", duration, aspect, resolution)
	}
	return fmt.Sprintf("firefly-veo31-%s-%ds-%s-%s", mode, duration, aspect, resolution)
}

func fixedVeoModel(effective dto.SpottedFrogModelMap, mode string, duration int, aspect, resolution string) (string, bool) {
	switch {
	case mode == "fast" && duration == 8 && aspect == "16x9" && resolution == "1080p":
		return effective.VeoFast16x98s1080p, true
	case mode == "fast" && duration == 8 && aspect == "9x16" && resolution == "1080p":
		return effective.VeoFast9x168s1080p, true
	case mode == "standard" && duration == 8 && aspect == "16x9" && resolution == "1080p":
		return effective.VeoStd16x98s1080p, true
	case mode == "standard" && duration == 8 && aspect == "9x16" && resolution == "1080p":
		return effective.VeoStd9x168s1080p, true
	case mode == "ref" && duration == 8 && aspect == "16x9" && resolution == "1080p":
		return effective.VeoRef16x98s1080p, true
	case mode == "ref" && duration == 8 && aspect == "9x16" && resolution == "1080p":
		return effective.VeoRef9x168s1080p, true
	default:
		return "", false
	}
}

func normalizedImages(req *relaycommon.TaskSubmitReq, meta metadataPayload) []string {
	images := append([]string(nil), req.Images...)
	if len(images) == 0 && strings.TrimSpace(req.Image) != "" {
		images = append(images, strings.TrimSpace(req.Image))
	}
	if len(images) == 0 && strings.TrimSpace(meta.Image) != "" {
		images = append(images, strings.TrimSpace(meta.Image))
	}
	if len(images) == 0 && len(meta.Images) > 0 {
		images = append(images, meta.Images...)
	}
	if len(images) == 0 && len(meta.ReferenceImages) > 0 {
		images = append(images, meta.ReferenceImages...)
	}
	if len(images) == 0 && strings.TrimSpace(req.InputReference) != "" && strings.TrimSpace(req.InputReference) != "[]" {
		images = append(images, strings.TrimSpace(req.InputReference))
	}
	return images
}

func countMultipartReferenceFiles(c *gin.Context) int {
	files, err := collectMultipartReferenceFiles(c)
	if err != nil {
		return 0
	}
	return len(files)
}

func collectMultipartReferenceFiles(c *gin.Context) ([]omniInputFile, error) {
	if c == nil || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type"))), "multipart/form-data") {
		return nil, nil
	}
	form := c.Request.MultipartForm
	if form == nil {
		var err error
		form, err = common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, err
		}
	}
	fieldNames := []string{"input_reference[]", "input_reference", "images", "image"}
	var files []omniInputFile
	for _, fieldName := range fieldNames {
		fileHeaders := form.File[fieldName]
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				return nil, err
			}
			contentType := normalizeContentType(fileHeader.Header.Get("Content-Type"))
			if contentType == "" || contentType == "application/octet-stream" {
				contentType = http.DetectContentType(data)
			}
			filename := strings.TrimSpace(fileHeader.Filename)
			if filename == "" {
				filename = omniGeneratedFilename(len(files)+1, contentType)
			}
			files = append(files, omniInputFile{
				Filename:    filename,
				ContentType: contentType,
				Data:        data,
			})
		}
	}
	return files, nil
}

func collectOmniInputFiles(c *gin.Context, req *relaycommon.TaskSubmitReq, meta metadataPayload) ([]omniInputFile, error) {
	files, err := collectMultipartReferenceFiles(c)
	if err != nil {
		return nil, err
	}
	if len(files) > 0 {
		return files, nil
	}

	refs := normalizedOmniImageRefs(req, meta)
	if len(refs) == 0 {
		return nil, nil
	}
	files = make([]omniInputFile, 0, len(refs))
	for idx, ref := range refs {
		file, err := omniInputFileFromString(ref, idx+1)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func normalizedOmniImageRefs(req *relaycommon.TaskSubmitReq, meta metadataPayload) []string {
	refs := make([]string, 0, len(req.Images)+len(meta.ReferenceImages)+len(meta.Images)+2)
	appendRef := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		refs = append(refs, value)
	}
	appendRefs := func(values []string) {
		for _, value := range values {
			appendRef(value)
		}
	}

	appendRefs(req.Images)
	appendRef(req.Image)
	appendRefs(meta.ReferenceImages)
	appendRefs(meta.Images)
	appendRef(meta.Image)
	appendRefs(parseInputReferenceValues(req.InputReference))

	return refs
}

func parseInputReferenceValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var values []string
		if err := common.UnmarshalJsonStr(raw, &values); err == nil {
			result := make([]string, 0, len(values))
			for _, value := range values {
				value = strings.TrimSpace(value)
				if value != "" {
					result = append(result, value)
				}
			}
			return result
		}
	}
	return []string{raw}
}

func omniInputFileFromString(raw string, index int) (omniInputFile, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return omniInputFile{}, fmt.Errorf("empty omni input reference")
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return omniInputFileFromURL(raw, index)
	}
	return omniInputFileFromBase64(raw, index)
}

func omniInputFileFromURL(rawURL string, index int) (omniInputFile, error) {
	resp, err := service.DoDownloadRequest(rawURL, "spottedfrog_omni_input_reference")
	if err != nil {
		return omniInputFile{}, fmt.Errorf("download omni input reference failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return omniInputFile{}, fmt.Errorf("download omni input reference failed: http %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return omniInputFile{}, err
	}
	contentType := normalizeContentType(resp.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	filename := omniFilenameFromURL(rawURL, index, contentType)
	return omniInputFile{
		Filename:    filename,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func omniInputFileFromBase64(raw string, index int) (omniInputFile, error) {
	contentType, cleanBase64, err := service.DecodeBase64FileData(raw)
	if err != nil {
		return omniInputFile{}, fmt.Errorf("decode omni input reference failed: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(cleanBase64)
	if err != nil {
		return omniInputFile{}, fmt.Errorf("decode omni input reference bytes failed: %w", err)
	}
	contentType = normalizeContentType(contentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	return omniInputFile{
		Filename:    omniGeneratedFilename(index, contentType),
		ContentType: contentType,
		Data:        data,
	}, nil
}

func writeOmniMultipartFile(writer *multipart.Writer, index int, file omniInputFile) error {
	contentType := normalizeContentType(file.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(file.Data)
	}
	filename := strings.TrimSpace(file.Filename)
	if filename == "" {
		filename = omniGeneratedFilename(index+1, contentType)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, "input_reference[]", filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = part.Write(file.Data)
	return err
}

func omniGenerateAudio(meta metadataPayload) bool {
	if meta.GenerateAudio != nil {
		return *meta.GenerateAudio
	}
	if meta.Audio != nil {
		return *meta.Audio
	}
	return false
}

func omniSeconds(req *relaycommon.TaskSubmitReq) string {
	if strings.TrimSpace(req.Seconds) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil {
			if parsed >= 10 {
				return "10"
			}
			return "8"
		}
	}
	if req.Duration >= 10 {
		return "10"
	}
	return "8"
}

func omniSize(size string, meta metadataPayload) string {
	switch requestAspectRatio(size, meta) {
	case "9:16":
		return "1080x1920"
	case "1:1":
		return "1080x1080"
	default:
		return "1920x1080"
	}
}

func isOmniFlashRequest(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) bool {
	candidates := []string{}
	if info != nil {
		candidates = append(candidates, info.UpstreamModelName, info.OriginModelName)
	}
	if req != nil {
		candidates = append(candidates, req.Model)
	}
	for _, candidate := range candidates {
		switch normalizeVideoModelName(candidate) {
		case "omni", "omni_flash":
			return true
		}
	}
	return false
}

func normalizeVideoModelName(modelName string) string {
	return strings.ToLower(strings.TrimSpace(modelName))
}

func setSpottedFrogRequestContentType(c *gin.Context, contentType string) {
	if c == nil {
		return
	}
	c.Set(spottedFrogRequestContentTypeKey, contentType)
}

func normalizeContentType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return strings.TrimSpace(contentType)
	}
	return mediaType
}

func omniFilenameFromURL(rawURL string, index int, contentType string) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		base := strings.TrimSpace(path.Base(parsed.Path))
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	return omniGeneratedFilename(index, contentType)
}

func omniGeneratedFilename(index int, contentType string) string {
	ext := ".bin"
	if exts, err := mime.ExtensionsByType(normalizeContentType(contentType)); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	return fmt.Sprintf("input_reference_%d%s", index, ext)
}

func boolPtr(value bool) *bool {
	return &value
}

func requestDuration(req *relaycommon.TaskSubmitReq) (int, bool) {
	if strings.TrimSpace(req.Seconds) != "" {
		seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
		if err == nil {
			return seconds, true
		}
	}
	if req.SecondsSet {
		return 0, true
	}
	if req.DurationSet {
		return req.Duration, true
	}
	if req.Duration > 0 {
		return req.Duration, true
	}
	return 0, false
}

func requestAspectRatio(size string, meta metadataPayload) string {
	if meta.AspectRatio != "" {
		return meta.AspectRatio
	}
	if meta.Ratio != "" {
		return meta.Ratio
	}
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1920x1080", "1280x720":
		return "16:9"
	case "1080x1920", "720x1280":
		return "9:16"
	case "1024x1024", "1080x1080", "512x512":
		return "1:1"
	case "1792x1024":
		return "3:2"
	case "1024x1792":
		return "2:3"
	default:
		return ratioFromSize(size)
	}
}

func ratioFromSize(size string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) != 2 {
		return ""
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	if w <= 0 || h <= 0 {
		return ""
	}
	g := greatestCommonDivisor(w, h)
	if g <= 0 {
		return ""
	}
	switch fmt.Sprintf("%d:%d", w/g, h/g) {
	case "1:1", "16:9", "9:16", "3:2", "2:3":
		return fmt.Sprintf("%d:%d", w/g, h/g)
	default:
		return ""
	}
}

func aspectToken(ratio string) string {
	ratio = strings.ToLower(strings.TrimSpace(ratio))
	ratio = strings.ReplaceAll(ratio, ":", "x")
	switch ratio {
	case "16x9", "9x16", "1x1", "3x2", "2x3":
		return ratio
	default:
		return ""
	}
}

func greatestCommonDivisor(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func resolutionToken(resolution, size string) string {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	if resolution != "" {
		return resolution
	}
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) != 2 {
		return ""
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	maxDim := w
	if h > maxDim {
		maxDim = h
	}
	if maxDim >= 3840 {
		return "4k"
	}
	if maxDim >= 1920 {
		return "1080p"
	}
	if maxDim >= 1280 {
		return "720p"
	}
	return ""
}

func sanitizedMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	copied := make(map[string]any, len(metadata))
	for k, v := range metadata {
		if k == "model" {
			continue
		}
		copied[k] = v
	}
	if len(copied) == 0 {
		return nil
	}
	return copied
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func (r responsePayload) upstreamID() string {
	if strings.TrimSpace(r.ID) != "" {
		return strings.TrimSpace(r.ID)
	}
	return strings.TrimSpace(r.TaskID)
}

func (r responsePayload) resultURL() string {
	for _, value := range []string{r.URL, r.VideoURL, r.ResultURL, r.OutputURL} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	for _, values := range [][]string{r.URLs, r.ResultURLs} {
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func taskResultURL(task *model.Task, sfResp responsePayload) string {
	if url := strings.TrimSpace(task.GetResultURL()); url != "" {
		return url
	}
	return sfResp.resultURL()
}

func secondsString(r responsePayload) string {
	for _, value := range []any{r.Seconds, r.Duration} {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			if v == float64(int(v)) {
				return strconv.Itoa(int(v))
			}
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			return strconv.Itoa(v)
		}
	}
	return ""
}

func progressString(progress any) string {
	switch v := progress.(type) {
	case string:
		return v
	case float64:
		if v <= 0 {
			return ""
		}
		if v <= 1 {
			v *= 100
		}
		return fmt.Sprintf("%d%%", int(v))
	case int:
		if v <= 0 {
			return ""
		}
		return fmt.Sprintf("%d%%", v)
	default:
		return ""
	}
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
	return ""
}
