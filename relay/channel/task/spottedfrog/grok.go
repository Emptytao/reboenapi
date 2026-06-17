package spottedfrog

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const grokDefaultUpstreamModel = "grok-imagine-video-1.5-preview"

type grokImageURLPayload struct {
	URL string `json:"url"`
}

type grokImageReferenceItem struct {
	Type     string              `json:"type"`
	ImageURL grokImageURLPayload `json:"image_url"`
}

type grokRequestPayload struct {
	Model          string                   `json:"model"`
	Prompt         string                   `json:"prompt,omitempty"`
	ImageURL       string                   `json:"image_url,omitempty"`
	ImageReference []grokImageReferenceItem `json:"image_reference,omitempty"`
	Seconds        string                   `json:"seconds"`
	Size           string                   `json:"size"`
	ResolutionName string                   `json:"resolution_name"`
}

func isGrokModelName(modelName string) bool {
	switch normalizeVideoModelName(modelName) {
	case "grok", "grok-imagine-video", "grok-imagine-video-1.5-preview":
		return true
	default:
		return false
	}
}

func isGrokRequest(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) bool {
	candidates := make([]string, 0, 4)
	if info != nil {
		candidates = append(candidates, info.UpstreamModelName, info.OriginModelName)
	}
	if req != nil {
		candidates = append(candidates, req.Model)
	}
	for _, candidate := range candidates {
		if isGrokModelName(candidate) {
			return true
		}
	}
	return false
}

func grokReferenceCount(c *gin.Context, req *relaycommon.TaskSubmitReq, meta metadataPayload) int {
	files, err := collectMultipartReferenceFiles(c)
	if err == nil && len(files) > 0 {
		return len(files)
	}
	return len(grokRawImageReferences(req, meta))
}

func (a *TaskAdaptor) buildGrokRequestBody(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (io.Reader, error) {
	var meta metadataPayload
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &meta); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	imageURLs, err := collectGrokImageURLs(c, req, meta)
	if err != nil {
		return nil, err
	}
	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("grok-imagine-video requires at least one reference image")
	}

	seconds, err := grokSeconds(req)
	if err != nil {
		return nil, err
	}
	size, err := grokSize(req, meta)
	if err != nil {
		return nil, err
	}
	resolutionName, err := grokResolutionName(meta)
	if err != nil {
		return nil, err
	}

	body := grokRequestPayload{
		Model:          grokUpstreamModelName(taskcommon.DefaultString(info.UpstreamModelName, req.Model)),
		Prompt:         req.Prompt,
		Seconds:        seconds,
		Size:           size,
		ResolutionName: resolutionName,
	}
	if len(imageURLs) == 1 {
		body.ImageURL = imageURLs[0]
	} else {
		body.ImageReference = make([]grokImageReferenceItem, 0, len(imageURLs))
		for _, imageURL := range imageURLs {
			body.ImageReference = append(body.ImageReference, grokImageReferenceItem{
				Type: "image_url",
				ImageURL: grokImageURLPayload{
					URL: imageURL,
				},
			})
		}
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	setSpottedFrogRequestContentType(c, "application/json")
	return bytes.NewReader(data), nil
}

func grokUpstreamModelName(modelName string) string {
	normalized := normalizeVideoModelName(modelName)
	switch normalized {
	case "", "grok", "grok-imagine-video", "grok-imagine-video-1.5-preview":
		return grokDefaultUpstreamModel
	default:
		return strings.TrimSpace(modelName)
	}
}

func collectGrokImageURLs(c *gin.Context, req *relaycommon.TaskSubmitReq, meta metadataPayload) ([]string, error) {
	files, err := collectMultipartReferenceFiles(c)
	if err != nil {
		return nil, err
	}
	if len(files) > 0 {
		urls := make([]string, 0, len(files))
		for _, file := range files {
			filename, err := storeImageBytes(file.Data, file.ContentType)
			if err != nil {
				return nil, err
			}
			publicURL, err := publicImageURL(filename)
			if err != nil {
				return nil, err
			}
			urls = append(urls, publicURL)
		}
		return urls, nil
	}

	rawRefs := grokRawImageReferences(req, meta)
	if len(rawRefs) == 0 {
		return nil, nil
	}
	urls := make([]string, 0, len(rawRefs))
	for _, rawRef := range rawRefs {
		url, err := storeImageFromString(rawRef)
		if err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func grokRawImageReferences(req *relaycommon.TaskSubmitReq, meta metadataPayload) []string {
	refs := make([]string, 0, len(req.Images)+len(meta.ReferenceImages)+len(meta.Images)+2)
	seen := make(map[string]struct{})
	appendRef := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "[]" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs = append(refs, value)
	}
	appendRefs := func(values []string) {
		for _, value := range values {
			appendRef(value)
		}
	}

	if len(req.Images) > 0 {
		appendRefs(req.Images)
	} else if strings.TrimSpace(req.Image) != "" {
		appendRef(req.Image)
	} else if len(meta.Images) > 0 {
		appendRefs(meta.Images)
	} else if strings.TrimSpace(meta.Image) != "" {
		appendRef(meta.Image)
	} else if len(meta.ReferenceImages) > 0 {
		appendRefs(meta.ReferenceImages)
	} else {
		appendRefs(parseInputReferenceValues(req.InputReference))
	}

	return refs
}

func grokSeconds(req *relaycommon.TaskSubmitReq) (string, error) {
	if strings.TrimSpace(req.Seconds) != "" {
		seconds := strings.TrimSpace(req.Seconds)
		if seconds != "10" && seconds != "15" {
			return "", fmt.Errorf("grok-imagine-video seconds must be 10 or 15")
		}
		return seconds, nil
	}
	if req.SecondsSet {
		return "", fmt.Errorf("grok-imagine-video seconds must be 10 or 15")
	}
	if req.DurationSet || req.Duration > 0 {
		if req.Duration != 10 && req.Duration != 15 {
			return "", fmt.Errorf("grok-imagine-video duration must be 10 or 15")
		}
		return strconv.Itoa(req.Duration), nil
	}
	return "10", nil
}

func grokSize(req *relaycommon.TaskSubmitReq, meta metadataPayload) (string, error) {
	size := strings.ToLower(strings.TrimSpace(req.Size))
	if normalizedSize, ok := grokSupportedSize(size); ok {
		return normalizedSize, nil
	}
	if req.Width != nil && req.Height != nil && *req.Width > 0 && *req.Height > 0 {
		if aspect := grokAspectFromDimensions(*req.Width, *req.Height); aspect != "" {
			return grokSizeFromAspect(aspect)
		}
		return "", fmt.Errorf("grok-imagine-video unsupported width/height: %dx%d", *req.Width, *req.Height)
	}
	if size != "" {
		if aspect := grokAspectFromSize(size); aspect != "" {
			return grokSizeFromAspect(aspect)
		}
		return "", fmt.Errorf("grok-imagine-video unsupported size: %s", req.Size)
	}
	if aspect := strings.TrimSpace(taskcommon.DefaultString(meta.AspectRatio, meta.Ratio)); aspect != "" {
		return grokSizeFromAspect(aspect)
	}
	return "1280x720", nil
}

func grokResolutionName(meta metadataPayload) (string, error) {
	resolution := strings.ToLower(strings.TrimSpace(meta.ResolutionName))
	if resolution == "" {
		resolution = strings.ToLower(strings.TrimSpace(meta.Resolution))
	}
	if resolution == "" {
		return "720p", nil
	}
	switch resolution {
	case "480p", "720p":
		return resolution, nil
	default:
		return "", fmt.Errorf("grok-imagine-video resolution_name must be 480p or 720p")
	}
}

func grokSupportedSize(size string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "1280x720", "720x1280", "1024x1024", "1792x1024", "1024x1792":
		return strings.ToLower(strings.TrimSpace(size)), true
	default:
		return "", false
	}
}

func grokAspectFromSize(size string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(size)), "x", 2)
	if len(parts) != 2 {
		return ""
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return ""
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return ""
	}
	return grokAspectFromDimensions(width, height)
}

func grokAspectFromDimensions(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	g := greatestCommonDivisor(width, height)
	if g <= 0 {
		return ""
	}
	switch fmt.Sprintf("%d:%d", width/g, height/g) {
	case "16:9", "9:16", "1:1", "3:2", "2:3":
		return fmt.Sprintf("%d:%d", width/g, height/g)
	default:
		return ""
	}
}

func grokSizeFromAspect(aspect string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(aspect)) {
	case "16:9":
		return "1280x720", nil
	case "9:16":
		return "720x1280", nil
	case "1:1":
		return "1024x1024", nil
	case "3:2":
		return "1792x1024", nil
	case "2:3":
		return "1024x1792", nil
	default:
		return "", fmt.Errorf("grok-imagine-video unsupported aspect ratio: %s", aspect)
	}
}
