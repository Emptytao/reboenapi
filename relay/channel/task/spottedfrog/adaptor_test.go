package spottedfrog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

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
			got, err := mapUpstreamModel(&tt.req, tt.modelName)
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

func TestConvertToRequestPayload(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "veo"}}
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
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "veo"}}
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
