package common

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/sjson"
)

type VideoModelRoutingRuleSet struct {
	Version int                     `json:"version,omitempty"`
	Rules   []VideoModelRoutingRule `json:"rules,omitempty"`
}

type VideoModelRoutingRule struct {
	Name        string               `json:"name,omitempty"`
	Enabled     *bool                `json:"enabled,omitempty"`
	Logic       string               `json:"logic,omitempty"`
	Conditions  []ConditionOperation `json:"conditions,omitempty"`
	TargetModel string               `json:"target_model,omitempty"`
	Operations  []ParamOperation     `json:"operations,omitempty"`
}

var videoModelRoutingHeaderModes = map[string]struct{}{
	"set_header":    {},
	"delete_header": {},
	"copy_header":   {},
	"move_header":   {},
	"pass_headers":  {},
}

func ApplyVideoModelRoutingWithRelayInfo(c *gin.Context, info *RelayInfo) (bool, error) {
	req, err := GetTaskRequest(c)
	if err != nil {
		return false, err
	}
	applied, nextReq, err := ApplyVideoModelRouting(info, req)
	if err != nil {
		return false, err
	}
	if applied {
		c.Set("task_request", nextReq)
	}
	return applied, nil
}

func ApplyVideoModelRouting(info *RelayInfo, req TaskSubmitReq) (bool, TaskSubmitReq, error) {
	if info == nil || info.ChannelMeta == nil {
		return false, req, nil
	}
	rawRules := strings.TrimSpace(info.ChannelMeta.ModelRoutingRules)
	if rawRules == "" || rawRules == "{}" {
		return false, req, nil
	}

	var ruleSet VideoModelRoutingRuleSet
	if err := common.Unmarshal([]byte(rawRules), &ruleSet); err != nil {
		return false, req, fmt.Errorf("unmarshal model routing rules failed: %w", err)
	}
	if len(ruleSet.Rules) == 0 {
		return false, req, nil
	}

	requestJSON, err := common.Marshal(req)
	if err != nil {
		return false, req, err
	}
	contextMap := BuildVideoModelRoutingContext(req, info)
	contextJSON, err := marshalContextJSON(contextMap)
	if err != nil {
		return false, req, err
	}

	for idx, rule := range ruleSet.Rules {
		if !isVideoModelRoutingRuleEnabled(rule) {
			continue
		}
		ok, err := checkConditions(requestJSON, contextJSON, rule.Conditions, rule.Logic)
		if err != nil {
			return false, req, fmt.Errorf("rule %d condition check failed: %w", idx, err)
		}
		if !ok {
			continue
		}

		workingJSON := requestJSON
		targetModel := strings.TrimSpace(rule.TargetModel)
		if targetModel != "" {
			workingJSON, err = sjson.SetBytes(workingJSON, "model", targetModel)
			if err != nil {
				return false, req, fmt.Errorf("rule %d set target model failed: %w", idx, err)
			}
		}
		if len(rule.Operations) > 0 {
			normalizedOps, err := normalizeVideoModelRoutingOperations(rule.Operations)
			if err != nil {
				return false, req, fmt.Errorf("rule %d operations invalid: %w", idx, err)
			}
			workingJSON, err = applyOperations(workingJSON, normalizedOps, contextMap)
			if err != nil {
				return false, req, fmt.Errorf("rule %d apply operations failed: %w", idx, err)
			}
		}

		var nextReq TaskSubmitReq
		if err := common.Unmarshal(workingJSON, &nextReq); err != nil {
			return false, req, fmt.Errorf("rule %d request decode failed: %w", idx, err)
		}
		if strings.TrimSpace(nextReq.Model) != "" {
			info.UpstreamModelName = strings.TrimSpace(nextReq.Model)
			info.IsModelMapped = true
		}
		return true, nextReq, nil
	}

	return false, req, nil
}

func ApplyTaskParamOverrideWithRelayInfo(c *gin.Context, info *RelayInfo) error {
	paramOverride := getParamOverrideMap(info)
	if len(paramOverride) == 0 {
		return nil
	}

	req, err := GetTaskRequest(c)
	if err != nil {
		return err
	}
	requestJSON, err := common.Marshal(req)
	if err != nil {
		return err
	}
	nextJSON, err := ApplyParamOverrideWithRelayInfo(requestJSON, info)
	if err != nil {
		return err
	}
	var nextReq TaskSubmitReq
	if err := common.Unmarshal(nextJSON, &nextReq); err != nil {
		return err
	}
	if modelName := strings.TrimSpace(nextReq.Model); modelName != "" {
		info.UpstreamModelName = modelName
	}
	c.Set("task_request", nextReq)
	return nil
}

func BuildVideoModelRoutingContext(req TaskSubmitReq, info *RelayInfo) map[string]interface{} {
	requestMap := make(map[string]interface{})
	requestJSON, err := common.Marshal(req)
	if err == nil {
		_ = common.Unmarshal(requestJSON, &requestMap)
	}

	derived := make(map[string]interface{})
	originalModel := strings.TrimSpace(req.Model)
	if originalModel == "" && info != nil {
		originalModel = strings.TrimSpace(info.OriginModelName)
	}
	if originalModel != "" {
		derived["original_model"] = originalModel
	}
	if req.Size != "" {
		derived["size"] = req.Size
	}
	if req.Width != nil {
		derived["width"] = *req.Width
	}
	if req.Height != nil {
		derived["height"] = *req.Height
	}
	if duration, ok := taskRequestDuration(req); ok {
		derived["duration"] = duration
	}
	if aspect := taskRequestAspectRatio(req); aspect != "" {
		derived["aspect_ratio"] = aspect
	}
	if resolution := taskRequestResolution(req); resolution != "" {
		derived["resolution"] = resolution
	}
	if mode := strings.TrimSpace(req.Mode); mode != "" {
		derived["mode"] = mode
	}
	imageCount := len(taskRequestImages(req))
	derived["image_count"] = imageCount
	derived["has_image"] = imageCount > 0

	ctx := make(map[string]interface{})
	if len(requestMap) > 0 {
		ctx["request"] = requestMap
	}
	ctx["derived"] = derived

	contextMeta := make(map[string]interface{})
	if info != nil {
		if info.RequestURLPath != "" {
			contextMeta["request_path"] = info.RequestURLPath
		}
		if info.TaskRelayInfo != nil && info.Action != "" {
			contextMeta["action"] = info.Action
		}
		if info.ChannelMeta != nil && info.ChannelType != 0 {
			contextMeta["channel_type"] = info.ChannelType
		}
	}
	ctx["context"] = contextMeta
	return ctx
}

func isVideoModelRoutingRuleEnabled(rule VideoModelRoutingRule) bool {
	return rule.Enabled == nil || *rule.Enabled
}

func normalizeVideoModelRoutingOperations(operations []ParamOperation) ([]ParamOperation, error) {
	normalized := make([]ParamOperation, 0, len(operations))
	for idx, op := range operations {
		mode := strings.ToLower(strings.TrimSpace(op.Mode))
		if mode == "" {
			return nil, fmt.Errorf("operation %d mode is required", idx)
		}
		if _, blocked := videoModelRoutingHeaderModes[mode]; blocked {
			return nil, fmt.Errorf("operation mode %s is not supported in video model routing", mode)
		}
		op.Mode = mode

		var err error
		op.Path, err = normalizeVideoModelRoutingPath(op.Path)
		if err != nil {
			return nil, fmt.Errorf("operation %d path invalid: %w", idx, err)
		}
		op.From, err = normalizeVideoModelRoutingPath(op.From)
		if err != nil {
			return nil, fmt.Errorf("operation %d from invalid: %w", idx, err)
		}
		op.To, err = normalizeVideoModelRoutingPath(op.To)
		if err != nil {
			return nil, fmt.Errorf("operation %d to invalid: %w", idx, err)
		}
		normalized = append(normalized, op)
	}
	return normalized, nil
}

func normalizeVideoModelRoutingPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "request.") {
		path = strings.TrimPrefix(path, "request.")
	}
	if strings.HasPrefix(path, "derived.") || strings.HasPrefix(path, "context.") {
		return "", fmt.Errorf("path %q must target request body fields", path)
	}
	return path, nil
}

func taskRequestDuration(req TaskSubmitReq) (int, bool) {
	if strings.TrimSpace(req.Seconds) != "" {
		if seconds := common.String2Int(strings.TrimSpace(req.Seconds)); seconds != 0 {
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

func taskRequestAspectRatio(req TaskSubmitReq) string {
	if ratio := taskRequestMetadataString(req.Metadata, "aspect_ratio"); ratio != "" {
		return normalizeRatioString(ratio)
	}
	if ratio := taskRequestMetadataString(req.Metadata, "ratio"); ratio != "" {
		return normalizeRatioString(ratio)
	}
	return normalizeRatioString(req.Size)
}

func taskRequestResolution(req TaskSubmitReq) string {
	if resolution := strings.ToLower(strings.TrimSpace(taskRequestMetadataString(req.Metadata, "resolution"))); resolution != "" {
		return resolution
	}
	size := strings.ToLower(strings.TrimSpace(req.Size))
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return ""
	}
	width := common.String2Int(parts[0])
	height := common.String2Int(parts[1])
	if width <= 0 || height <= 0 {
		return ""
	}
	maxDim := width
	if height > maxDim {
		maxDim = height
	}
	switch {
	case maxDim >= 3840:
		return "4k"
	case maxDim >= 1920:
		return "1080p"
	case maxDim >= 1280:
		return "720p"
	default:
		return ""
	}
}

func taskRequestImages(req TaskSubmitReq) []string {
	images := make([]string, 0, len(req.Images)+4)
	images = appendNonEmptyImages(images, req.Images...)
	if req.Image != "" {
		images = appendNonEmptyImages(images, req.Image)
	}
	if image := taskRequestMetadataString(req.Metadata, "image"); image != "" {
		images = appendNonEmptyImages(images, image)
	}
	if values := taskRequestMetadataStringSlice(req.Metadata, "images"); len(values) > 0 {
		images = appendNonEmptyImages(images, values...)
	}
	if values := taskRequestMetadataStringSlice(req.Metadata, "reference_images"); len(values) > 0 {
		images = appendNonEmptyImages(images, values...)
	}
	if req.InputReference != "" && strings.TrimSpace(req.InputReference) != "[]" {
		images = appendNonEmptyImages(images, req.InputReference)
	}
	return images
}

func taskRequestMetadataString(metadata map[string]interface{}, key string) string {
	if len(metadata) == 0 {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func taskRequestMetadataStringSlice(metadata map[string]interface{}, key string) []string {
	if len(metadata) == 0 {
		return nil
	}
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			trimmed := strings.TrimSpace(fmt.Sprintf("%v", item))
			if trimmed != "" {
				values = append(values, trimmed)
			}
		}
		return values
	default:
		return nil
	}
}

func appendNonEmptyImages(dst []string, values ...string) []string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			dst = append(dst, trimmed)
		}
	}
	return dst
}

func normalizeRatioString(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	switch value {
	case "16:9", "9:16", "1:1":
		return value
	}
	parts := strings.SplitN(value, "x", 2)
	if len(parts) == 2 {
		width := common.String2Int(parts[0])
		height := common.String2Int(parts[1])
		if width > 0 && height > 0 {
			switch {
			case width == height:
				return "1:1"
			case width > height:
				return "16:9"
			default:
				return "9:16"
			}
		}
	}
	return value
}
