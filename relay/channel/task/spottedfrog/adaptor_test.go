package spottedfrog

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func TestTaskSubmitReqStandardVideoFields(t *testing.T) {
	var req relaycommon.TaskSubmitReq
	if err := common.Unmarshal([]byte(`{"model":"sora-2","prompt":"city","duration":8,"width":1920,"height":1080,"fps":"24","seed":"7","n":"1","seconds":10}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.Size != "1920x1080" {
		t.Fatalf("Size = %q", req.Size)
	}
	if req.Duration != 8 || !req.DurationSet {
		t.Fatalf("Duration = %d DurationSet=%v", req.Duration, req.DurationSet)
	}
	if req.Seconds != "10" || !req.SecondsSet {
		t.Fatalf("Seconds = %q SecondsSet=%v", req.Seconds, req.SecondsSet)
	}
	if req.Fps == nil || *req.Fps != 24 {
		t.Fatalf("Fps = %v", req.Fps)
	}
	if req.Seed == nil || *req.Seed != 7 {
		t.Fatalf("Seed = %v", req.Seed)
	}
	if req.N == nil || *req.N != 1 {
		t.Fatalf("N = %v", req.N)
	}
}

func TestMapUpstreamModel(t *testing.T) {
	tests := []struct {
		name      string
		req       relaycommon.TaskSubmitReq
		modelName string
		want      string
		wantErr   bool
	}{
		{
			name:      "sora landscape 8s",
			modelName: "sora-2",
			req:       relaycommon.TaskSubmitReq{Model: "sora-2", Duration: 8, DurationSet: true, Size: "1920x1080"},
			want:      "sora-2-8s-16x9",
		},
		{
			name:      "sora portrait default duration",
			modelName: "sora-2",
			req:       relaycommon.TaskSubmitReq{Model: "sora-2", Size: "1080x1920"},
			want:      "sora-2-12s-9x16",
		},
		{
			name:      "sora pro portrait",
			modelName: "sora-2",
			req: relaycommon.TaskSubmitReq{
				Model:       "sora-2",
				Duration:    12,
				DurationSet: true,
				Size:        "1080x1920",
				Metadata:    map[string]interface{}{"variant": "pro"},
			},
			want: "sora2-pro-12s-9x16",
		},
		{
			name:      "sora pro rejects non 12s",
			modelName: "sora-2",
			req: relaycommon.TaskSubmitReq{
				Model:       "sora-2",
				Duration:    8,
				DurationSet: true,
				Size:        "1920x1080",
				Metadata:    map[string]interface{}{"variant": "pro"},
			},
			wantErr: true,
		},
		{
			name:      "omini stays same",
			modelName: "omni_flash",
			req:       relaycommon.TaskSubmitReq{Model: "omni_flash", Size: "1920x1080", Seconds: "10"},
			want:      "omni_flash",
		},
		{
			name:      "grok stays same",
			modelName: "grok-imagine-video",
			req:       relaycommon.TaskSubmitReq{Model: "grok-imagine-video", Size: "1920x1080"},
			want:      "grok-imagine-video",
		},
		{
			name:      "veo fast text",
			modelName: "veo",
			req:       relaycommon.TaskSubmitReq{Model: "veo", Duration: 8, DurationSet: true, Size: "1920x1080", Metadata: map[string]interface{}{"speed": "fast"}},
			want:      "firefly-veo31-fast-8s-16x9-1080p",
		},
		{
			name:      "veo reference",
			modelName: "veo",
			req:       relaycommon.TaskSubmitReq{Model: "veo", Duration: 8, DurationSet: true, Size: "1920x1080", Images: []string{"a", "b"}},
			want:      "firefly-veo31-ref-8s-16x9-1080p",
		},
		{
			name:      "veo metadata reference",
			modelName: "veo",
			req: relaycommon.TaskSubmitReq{
				Model:       "veo",
				Duration:    8,
				DurationSet: true,
				Size:        "1080x1920",
				Metadata:    map[string]interface{}{"reference_images": []string{"a"}},
			},
			want: "firefly-veo31-ref-8s-9x16-1080p",
		},
		{
			name:      "omini rejects too many references",
			modelName: "omni_flash",
			req:       relaycommon.TaskSubmitReq{Model: "omni_flash", Images: []string{"1", "2", "3", "4", "5", "6", "7", "8"}},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapUpstreamModel(&tt.req, tt.modelName, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("model = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapUpstreamModelConfigurableOverrides(t *testing.T) {
	overrides := &dto.SpottedFrogModelMap{
		Sora216x98s:        "custom-sora-landscape-8s",
		OmniFlash:          "custom-omni-flash",
		GrokImagineVideo:   "custom-grok-video",
		VeoRef16x98s1080p:  "custom-veo-ref",
		VeoFast16x98s1080p: "custom-veo-fast",
	}

	tests := []struct {
		name      string
		req       relaycommon.TaskSubmitReq
		modelName string
		want      string
	}{
		{
			name:      "sora override",
			modelName: "sora-2",
			req:       relaycommon.TaskSubmitReq{Model: "sora-2", Duration: 8, DurationSet: true, Size: "1920x1080"},
			want:      "custom-sora-landscape-8s",
		},
		{
			name:      "omni override",
			modelName: "omni_flash",
			req:       relaycommon.TaskSubmitReq{Model: "omni_flash"},
			want:      "custom-omni-flash",
		},
		{
			name:      "grok override",
			modelName: "grok-imagine-video",
			req:       relaycommon.TaskSubmitReq{Model: "grok-imagine-video"},
			want:      "custom-grok-video",
		},
		{
			name:      "veo fixed override",
			modelName: "veo",
			req:       relaycommon.TaskSubmitReq{Model: "veo", Duration: 8, DurationSet: true, Size: "1920x1080", Images: []string{"https://example.com/ref.jpg"}},
			want:      "custom-veo-ref",
		},
		{
			name:      "veo fast override",
			modelName: "veo",
			req:       relaycommon.TaskSubmitReq{Model: "veo", Duration: 8, DurationSet: true, Size: "1920x1080"},
			want:      "custom-veo-fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapUpstreamModel(&tt.req, tt.modelName, overrides)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("model = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapUpstreamModelPartialOverrideFallsBackToDefaults(t *testing.T) {
	got, err := mapUpstreamModel(
		&relaycommon.TaskSubmitReq{
			Model:       "sora-2",
			Duration:    12,
			DurationSet: true,
			Size:        "1080x1920",
		},
		"sora-2",
		&dto.SpottedFrogModelMap{
			Sora216x98s: "custom-sora-landscape-8s",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sora-2-12s-9x16" {
		t.Fatalf("model = %q", got)
	}
}

func TestMapUpstreamModelVeoNonFixedFallback(t *testing.T) {
	got, err := mapUpstreamModel(
		&relaycommon.TaskSubmitReq{
			Model:       "veo",
			Duration:    12,
			DurationSet: true,
			Size:        "1920x1080",
			Metadata:    map[string]interface{}{"speed": "fast"},
		},
		"veo",
		&dto.SpottedFrogModelMap{
			VeoFast16x98s1080p: "custom-veo-fast",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "firefly-veo31-fast-12s-16x9-1080p" {
		t.Fatalf("model = %q", got)
	}
}

func TestApplyModelMappingUsesMappedLogicalModelAndChannelOverrides(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "sora-2",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				SpottedFrogModelMap: &dto.SpottedFrogModelMap{
					Sora216x94s: "channel-specific-sora",
				},
			},
		},
	}
	req := relaycommon.TaskSubmitReq{
		Model:       "public-sora",
		Duration:    4,
		DurationSet: true,
		Size:        "1920x1080",
	}
	if err := applyModelMapping(nil, info, &req); err != nil {
		t.Fatal(err)
	}
	if info.UpstreamModelName != "channel-specific-sora" {
		t.Fatalf("UpstreamModelName = %q", info.UpstreamModelName)
	}
}

func TestConvertToRequestPayload(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "firefly-veo31-ref-8s-16x9-1080p"}}
	req := relaycommon.TaskSubmitReq{
		Model:       "veo",
		Prompt:      "transition",
		Size:        "1920x1080",
		Duration:    8,
		DurationSet: true,
		Images:      []string{"https://example.com/first.jpg", "https://example.com/last.jpg"},
		Metadata:    map[string]interface{}{"generate_audio": false},
	}
	got, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "firefly-veo31-ref-8s-16x9-1080p" {
		t.Fatalf("Model = %q", got.Model)
	}
	if len(got.ReferenceImages) != 2 || got.ReferenceMode != "image" {
		t.Fatalf("ReferenceImages = %#v ReferenceMode=%q", got.ReferenceImages, got.ReferenceMode)
	}
	if got.GenerateAudio == nil || *got.GenerateAudio {
		t.Fatalf("GenerateAudio = %v", got.GenerateAudio)
	}
}

func TestConvertToRequestPayloadVeoSingleReferenceUsesReferenceImages(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "firefly-veo31-ref-8s-16x9-1080p"}}
	req := relaycommon.TaskSubmitReq{
		Model:       "veo",
		Prompt:      "portrait",
		Size:        "1920x1080",
		Duration:    8,
		DurationSet: true,
		Images:      []string{"https://example.com/ref.jpg"},
	}
	got, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "firefly-veo31-ref-8s-16x9-1080p" {
		t.Fatalf("Model = %q", got.Model)
	}
	if len(got.ReferenceImages) != 1 || got.ReferenceImages[0] != "https://example.com/ref.jpg" {
		t.Fatalf("ReferenceImages = %#v", got.ReferenceImages)
	}
	if got.Image != "" {
		t.Fatalf("Image = %q", got.Image)
	}
}

func TestBuildOmniTextRequestBodyJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:       "omni_flash",
		Prompt:      "提示词",
		Size:        "1280x720",
		Duration:    4,
		DurationSet: true,
		Metadata: map[string]interface{}{
			"aspect_ratio":   "16:9",
			"generate_audio": false,
		},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "omni_flash"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := common.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "omni_flash" {
		t.Fatalf("model = %#v", payload["model"])
	}
	if payload["size"] != "1920x1080" {
		t.Fatalf("size = %#v", payload["size"])
	}
	if payload["seconds"] != "8" {
		t.Fatalf("seconds = %#v", payload["seconds"])
	}
	if payload["input_reference"] != "[]" {
		t.Fatalf("input_reference = %#v", payload["input_reference"])
	}
	for _, unexpected := range []string{"generate_audio", "duration", "aspect_ratio", "metadata", "reference_images", "reference_mode"} {
		if _, exists := payload[unexpected]; exists {
			t.Fatalf("unexpected field %q in payload: %#v", unexpected, payload[unexpected])
		}
	}

	upstreamReq, err := http.NewRequest(http.MethodPost, "https://api.hellobabygo.com/v1/videos?async=true", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.BuildRequestHeader(c, upstreamReq, info); err != nil {
		t.Fatal(err)
	}
	if got := upstreamReq.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := upstreamReq.Header.Get("Prefer"); got != "respond-async" {
		t.Fatalf("Prefer = %q", got)
	}
}

func TestBuildOmniTextRequestBodyIncludesGenerateAudioWhenEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:    "omni_flash",
		Prompt:   "带音频",
		Size:     "1080x1920",
		Seconds:  "10",
		Metadata: map[string]interface{}{"audio": true},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "omni_flash"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := common.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["size"] != "1080x1920" {
		t.Fatalf("size = %#v", payload["size"])
	}
	if payload["seconds"] != "10" {
		t.Fatalf("seconds = %#v", payload["seconds"])
	}
	if payload["generate_audio"] != true {
		t.Fatalf("generate_audio = %#v", payload["generate_audio"])
	}
}

func TestGrokRawImageReferencesMergesAllCompatibleSources(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Images:         []string{"https://example.com/one.png"},
		Image:          "https://example.com/two.png",
		InputReference: `["https://example.com/three.png", "https://example.com/one.png"]`,
		Metadata: map[string]interface{}{
			"images":           []string{"https://example.com/four.png"},
			"image":            "https://example.com/five.png",
			"reference_images": []string{"https://example.com/six.png"},
		},
	}
	var meta metadataPayload
	if err := taskcommon.UnmarshalMetadata(req.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	got := grokRawImageReferences(&req, meta)
	want := []string{
		"https://example.com/one.png",
		"https://example.com/two.png",
		"https://example.com/four.png",
		"https://example.com/five.png",
		"https://example.com/six.png",
		"https://example.com/three.png",
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want=%d got=%#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want=%q full=%#v", i, got[i], want[i], got)
		}
	}
}

func TestIsGrokRequestRecognizesConfiguredOverride(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				SpottedFrogModelMap: &dto.SpottedFrogModelMap{
					GrokImagineVideo: "custom-grok-video",
				},
			},
		},
	}
	req := relaycommon.TaskSubmitReq{Model: "custom-grok-video"}
	if !isGrokRequest(&req, info) {
		t.Fatal("expected configured grok model to be recognized")
	}
}

func TestBuildGrokSingleImageRequestBodyJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:    "grok-imagine-video",
		Prompt:   "single image",
		Seconds:  "10",
		Size:     "720x1280",
		Image:    "https://example.com/reference.jpg",
		Metadata: map[string]interface{}{"resolution_name": "720p"},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	var payload grokRequestPayload
	if err := common.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Model != "grok-imagine-video-1.5-preview" {
		t.Fatalf("model = %q", payload.Model)
	}
	if payload.ImageURL != "https://example.com/reference.jpg" {
		t.Fatalf("image_url = %q", payload.ImageURL)
	}
	if len(payload.ImageReference) != 0 {
		t.Fatalf("image_reference = %#v", payload.ImageReference)
	}
	if payload.Seconds != "10" {
		t.Fatalf("seconds = %q", payload.Seconds)
	}
	if payload.Size != "720x1280" {
		t.Fatalf("size = %q", payload.Size)
	}
	if payload.ResolutionName != "720p" {
		t.Fatalf("resolution_name = %q", payload.ResolutionName)
	}

	upstreamURL, err := a.BuildRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamURL != "https://api.hellobabygo.com/v1/videos" {
		t.Fatalf("BuildRequestURL = %q", upstreamURL)
	}
	upstreamReq, err := http.NewRequest(http.MethodPost, upstreamURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.BuildRequestHeader(c, upstreamReq, info); err != nil {
		t.Fatal(err)
	}
	if got := upstreamReq.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := upstreamReq.Header.Get("Prefer"); got != "" {
		t.Fatalf("Prefer should be empty, got %q", got)
	}
}

func TestBuildGrokConfiguredOverrideRequestBodyUsesCustomModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:    "my-grok-alias",
		Prompt:   "custom override",
		Duration: 15,
		Image:    "https://example.com/override.jpg",
		Metadata: map[string]interface{}{"aspect_ratio": "16:9"},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		OriginModelName: "my-grok-alias",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "custom-grok-video",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				SpottedFrogModelMap: &dto.SpottedFrogModelMap{
					GrokImagineVideo: "custom-grok-video",
				},
			},
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	var payload grokRequestPayload
	if err := common.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Model != "custom-grok-video" {
		t.Fatalf("model = %q", payload.Model)
	}
	if payload.Seconds != "15" {
		t.Fatalf("seconds = %q", payload.Seconds)
	}
	if payload.Size != "1280x720" {
		t.Fatalf("size = %q", payload.Size)
	}

	upstreamURL, err := a.BuildRequestURL(info)
	if err != nil {
		t.Fatal(err)
	}
	if upstreamURL != "https://api.hellobabygo.com/v1/videos" {
		t.Fatalf("BuildRequestURL = %q", upstreamURL)
	}
}

func TestBuildGrokMultiImageRequestBodyStoresLocalImages(t *testing.T) {
	oldBaseURL := common.TaskImagePublicBaseURL
	common.TaskImagePublicBaseURL = "https://unit.test"
	defer func() {
		common.TaskImagePublicBaseURL = oldBaseURL
	}()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:   "grok-imagine-video",
		Prompt:  "multi image",
		Seconds: "10",
		Images: []string{
			"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4////fwAJ+wP9KobjigAAAABJRU5ErkJggg==",
			"https://example.com/external.png",
		},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		OriginModelName: "grok-imagine-video",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "grok-imagine-video",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	var payload grokRequestPayload
	if err := common.Unmarshal(bodyBytes, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ImageURL != "" {
		t.Fatalf("image_url should be empty for multi image payload, got %q", payload.ImageURL)
	}
	if len(payload.ImageReference) != 2 {
		t.Fatalf("image_reference len = %d", len(payload.ImageReference))
	}
	if !strings.HasPrefix(payload.ImageReference[0].ImageURL.URL, "https://unit.test/img/") {
		t.Fatalf("stored image url = %q", payload.ImageReference[0].ImageURL.URL)
	}
	if payload.ImageReference[1].ImageURL.URL != "https://example.com/external.png" {
		t.Fatalf("external image url = %q", payload.ImageReference[1].ImageURL.URL)
	}
	matches, err := filepath.Glob(filepath.Join(tmpDir, "img", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("stored files = %#v", matches)
	}
}

func TestFetchTaskUsesPlainEndpointForGrok(t *testing.T) {
	service.InitHttpClient()
	callPaths := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callPaths = append(callPaths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"grok-task-1","status":"queued"}`))
	}))
	defer server.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTask(server.URL, "sk-test", map[string]any{
		"task_id":      "grok-task-1",
		"origin_model": "grok-imagine-video",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(callPaths) != 1 {
		t.Fatalf("call paths = %#v", callPaths)
	}
	if callPaths[0] != "/v1/videos/grok-task-1" {
		t.Fatalf("call path = %q", callPaths[0])
	}
}

func TestBuildOmniMultipartRequestBodyFromDataURI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	req := relaycommon.TaskSubmitReq{
		Model:   "omni_flash",
		Prompt:  "图生视频",
		Size:    "720x1280",
		Seconds: "10",
		Images: []string{
			"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4////fwAJ+wP9KobjigAAAABJRU5ErkJggg==",
		},
		Metadata: map[string]interface{}{"generate_audio": false},
	}
	c.Set("task_request", req)

	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "omni_flash"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}

	bodyReader, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatal(err)
	}
	bodyBytes, err := io.ReadAll(bodyReader)
	if err != nil {
		t.Fatal(err)
	}

	upstreamReq, err := http.NewRequest(http.MethodPost, "https://api.hellobabygo.com/v1/videos?async=true", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.BuildRequestHeader(c, upstreamReq, info); err != nil {
		t.Fatal(err)
	}
	contentType := upstreamReq.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("media type = %q", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(bodyBytes), params["boundary"])
	fields := map[string][]string{}
	files := map[string]int{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() != "" {
			files[part.FormName()]++
			if part.FormName() != "input_reference[]" {
				t.Fatalf("unexpected file field %q", part.FormName())
			}
			if normalizeContentType(part.Header.Get("Content-Type")) != "image/png" {
				t.Fatalf("file content type = %q", part.Header.Get("Content-Type"))
			}
			if len(data) == 0 {
				t.Fatal("multipart file body is empty")
			}
			continue
		}
		fields[part.FormName()] = append(fields[part.FormName()], string(data))
	}
	if fields["model"][0] != "omni_flash" {
		t.Fatalf("model field = %#v", fields["model"])
	}
	if fields["size"][0] != "1080x1920" {
		t.Fatalf("size field = %#v", fields["size"])
	}
	if fields["seconds"][0] != "10" {
		t.Fatalf("seconds field = %#v", fields["seconds"])
	}
	if _, exists := fields["generate_audio"]; exists {
		t.Fatalf("generate_audio should be omitted when false: %#v", fields["generate_audio"])
	}
	if files["input_reference[]"] != 1 {
		t.Fatalf("input_reference[] file count = %d", files["input_reference[]"])
	}
}

func TestValidateRequestAndSetActionCountsMultipartFilesForOmni(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "omni_flash")
	_ = writer.WriteField("prompt", "multipart")
	_ = writer.WriteField("size", "1920x1080")
	_ = writer.WriteField("seconds", "8")
	part1, _ := writer.CreateFormFile("input_reference[]", "first.png")
	_, _ = part1.Write([]byte("first-image"))
	part2, _ := writer.CreateFormFile("input_reference[]", "last.png")
	_, _ = part2.Write([]byte("last-image"))
	_ = writer.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())

	info := &relaycommon.RelayInfo{
		ChannelMeta:   &relaycommon.ChannelMeta{UpstreamModelName: "omni_flash"},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	a := &TaskAdaptor{}
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("task error: %s", taskErr.Message)
	}
	if info.Action != constant.TaskActionFirstTailGenerate {
		t.Fatalf("action = %q", info.Action)
	}
}

func TestFetchTaskUsesAsyncQueryThenFallback(t *testing.T) {
	service.InitHttpClient()
	callPaths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callPaths = append(callPaths, r.URL.RequestURI())
		if strings.Contains(r.URL.RawQuery, "async=true") {
			http.Error(w, "try fallback", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"frog-task-1","status":"queued"}`))
	}))
	defer server.Close()

	a := &TaskAdaptor{}
	resp, err := a.FetchTask(server.URL, "sk-test", map[string]any{"task_id": "frog-task-1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if len(callPaths) != 2 {
		t.Fatalf("call paths = %#v", callPaths)
	}
	if callPaths[0] != "/v1/videos/frog-task-1?async=true" {
		t.Fatalf("first call path = %q", callPaths[0])
	}
	if callPaths[1] != "/v1/videos/frog-task-1" {
		t.Fatalf("second call path = %q", callPaths[1])
	}
}

func TestParseTaskResult(t *testing.T) {
	a := &TaskAdaptor{}
	tests := []struct {
		name       string
		body       string
		wantStatus model.TaskStatus
		wantURL    string
		wantReason string
	}{
		{name: "queued", body: `{"id":"t1","status":"queued"}`, wantStatus: model.TaskStatusQueued},
		{name: "processing", body: `{"id":"t1","status":"running","progress":0.5}`, wantStatus: model.TaskStatusInProgress},
		{name: "success url", body: `{"id":"t1","status":"completed","video_url":"https://cdn.example.com/out.mp4"}`, wantStatus: model.TaskStatusSuccess, wantURL: "https://cdn.example.com/out.mp4"},
		{name: "success urls fallback", body: `{"id":"t1","status":"success","urls":["","https://cdn.example.com/out2.mp4"]}`, wantStatus: model.TaskStatusSuccess, wantURL: "https://cdn.example.com/out2.mp4"},
		{name: "failure", body: `{"id":"t1","status":"failed","error":{"message":"bad prompt"}}`, wantStatus: model.TaskStatusFailure, wantReason: "bad prompt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.ParseTaskResult([]byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != string(tt.wantStatus) {
				t.Fatalf("Status = %q, want %q", got.Status, tt.wantStatus)
			}
			if got.Url != tt.wantURL {
				t.Fatalf("Url = %q, want %q", got.Url, tt.wantURL)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestDoResponseReturnsUpstreamIDAndPublicVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	a := &TaskAdaptor{}
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"id":"frog-task-1","status":"queued"}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "sora-2",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	taskID, taskData, taskErr := a.DoResponse(c, resp, info)
	if taskErr != nil {
		t.Fatal(taskErr.Message)
	}
	if taskID != "frog-task-1" {
		t.Fatalf("taskID = %q", taskID)
	}
	if len(taskData) == 0 {
		t.Fatal("taskData is empty")
	}

	var video dto.OpenAIVideo
	if err := common.Unmarshal(recorder.Body.Bytes(), &video); err != nil {
		t.Fatal(err)
	}
	if video.ID != "task_public" || video.TaskID != "task_public" {
		t.Fatalf("public ids = %q/%q", video.ID, video.TaskID)
	}
	if video.Model != "sora-2" {
		t.Fatalf("model = %q", video.Model)
	}
}

func TestConvertToOpenAIVideo(t *testing.T) {
	a := &TaskAdaptor{}
	task := &model.Task{
		TaskID:    "task_public",
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		CreatedAt: 100,
		UpdatedAt: 200,
		Properties: model.Properties{
			OriginModelName: "veo",
		},
		Data: []byte(`{"id":"frog-task-1","status":"completed","result_urls":["https://cdn.example.com/out.mp4"],"duration":8,"size":"1920x1080"}`),
	}
	data, err := a.ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatal(err)
	}
	var video dto.OpenAIVideo
	if err := common.Unmarshal(data, &video); err != nil {
		t.Fatal(err)
	}
	if video.ID != "task_public" {
		t.Fatalf("ID = %q", video.ID)
	}
	if video.Status != dto.VideoStatusCompleted {
		t.Fatalf("Status = %q", video.Status)
	}
	if video.Metadata["url"] != "https://cdn.example.com/out.mp4" {
		t.Fatalf("metadata.url = %#v", video.Metadata["url"])
	}
	if video.Seconds != "8" {
		t.Fatalf("Seconds = %q", video.Seconds)
	}
	if video.Size != "1920x1080" {
		t.Fatalf("Size = %q", video.Size)
	}
}
