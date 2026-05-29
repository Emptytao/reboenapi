package hkcopp

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

func TestConvertToRequestPayloadModeDerivation(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2-official"}}

	tests := []struct {
		name     string
		req      relaycommon.TaskSubmitReq
		wantMode string
		check    func(t *testing.T, got *requestPayload)
	}{
		{
			name:     "text to video",
			req:      relaycommon.TaskSubmitReq{Prompt: "city"},
			wantMode: "text_to_video",
		},
		{
			name:     "image to video",
			req:      relaycommon.TaskSubmitReq{Prompt: "walk", Images: []string{"https://example.com/a.png"}},
			wantMode: "image_to_video",
			check: func(t *testing.T, got *requestPayload) {
				if got.ImageURL != "https://example.com/a.png" {
					t.Fatalf("ImageURL = %q", got.ImageURL)
				}
			},
		},
		{
			name:     "first last frame",
			req:      relaycommon.TaskSubmitReq{Prompt: "transition", Images: []string{"https://example.com/a.png", "https://example.com/b.png"}},
			wantMode: "first_last_frame",
			check: func(t *testing.T, got *requestPayload) {
				if len(got.ImageURLs) != 2 {
					t.Fatalf("ImageURLs len = %d", len(got.ImageURLs))
				}
			},
		},
		{
			name:     "multi reference",
			req:      relaycommon.TaskSubmitReq{Prompt: "refs", Images: []string{"a", "b", "c"}},
			wantMode: "multi_ref",
		},
		{
			name:     "explicit mode wins",
			req:      relaycommon.TaskSubmitReq{Prompt: "motion", Mode: "motion_transfer", Images: []string{"a"}},
			wantMode: "motion_transfer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := a.convertToRequestPayload(&tt.req, info)
			if err != nil {
				t.Fatal(err)
			}
			if got.Mode != tt.wantMode {
				t.Fatalf("Mode = %q, want %q", got.Mode, tt.wantMode)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestConvertToRequestPayloadRatioMetadataAndZeroValues(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2-official"}}
	req := relaycommon.TaskSubmitReq{
		Prompt:   "product",
		Size:     "1280x720",
		Duration: 6,
		Metadata: map[string]interface{}{
			"ratio":          "4:3",
			"resolution":     "1080p",
			"webhook_url":    "https://example.com/hook",
			"watermark":      false,
			"generate_audio": false,
			"web_search":     true,
			"video_urls":     []string{"https://example.com/source.mp4"},
		},
	}

	got, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ratio != "4:3" {
		t.Fatalf("Ratio = %q", got.Ratio)
	}
	if got.Resolution != "1080p" {
		t.Fatalf("Resolution = %q", got.Resolution)
	}
	if got.WebhookURL != "https://example.com/hook" {
		t.Fatalf("WebhookURL = %q", got.WebhookURL)
	}
	if got.Duration == nil || *got.Duration != 6 {
		t.Fatalf("Duration = %v", got.Duration)
	}
	if got.Watermark == nil || *got.Watermark {
		t.Fatalf("Watermark = %v", got.Watermark)
	}
	if got.GenerateAudio == nil || *got.GenerateAudio {
		t.Fatalf("GenerateAudio = %v", got.GenerateAudio)
	}
	if got.WebSearch == nil || !*got.WebSearch {
		t.Fatalf("WebSearch = %v", got.WebSearch)
	}
	if len(got.VideoURLs) != 1 || got.VideoURLs[0] != "https://example.com/source.mp4" {
		t.Fatalf("VideoURLs = %#v", got.VideoURLs)
	}
}

func TestConvertToRequestPayloadSecondsZeroPreserved(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2-official"}}
	req := relaycommon.TaskSubmitReq{Prompt: "zero", Seconds: "0"}

	got, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Duration == nil || *got.Duration != 0 {
		t.Fatalf("Duration = %v, want pointer to 0", got.Duration)
	}
}

func TestConvertToRequestPayloadDurationZeroPreserved(t *testing.T) {
	a := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "seedance-2-official"}}
	var req relaycommon.TaskSubmitReq
	if err := common.Unmarshal([]byte(`{"prompt":"zero","duration":0}`), &req); err != nil {
		t.Fatal(err)
	}

	got, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Duration == nil || *got.Duration != 0 {
		t.Fatalf("Duration = %v, want pointer to 0", got.Duration)
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
		{name: "processing", body: `{"id":"t1","status":"running"}`, wantStatus: model.TaskStatusInProgress},
		{name: "success result url", body: `{"id":"t1","status":"completed","result_url":"https://cdn.example.com/out.mp4"}`, wantStatus: model.TaskStatusSuccess, wantURL: "https://cdn.example.com/out.mp4"},
		{name: "success result urls fallback", body: `{"id":"t1","status":"success","result_urls":["","https://cdn.example.com/out2.mp4"]}`, wantStatus: model.TaskStatusSuccess, wantURL: "https://cdn.example.com/out2.mp4"},
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
		Body: io.NopCloser(strings.NewReader(`{"id":"hkcopp-task-1","status":"queued"}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2-official",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	taskID, taskData, taskErr := a.DoResponse(c, resp, info)
	if taskErr != nil {
		t.Fatal(taskErr.Message)
	}
	if taskID != "hkcopp-task-1" {
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
	if video.Model != "seedance-2-official" {
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
			OriginModelName: "seedance-2-official",
		},
		Data: []byte(`{"id":"hkcopp-task-1","status":"completed","result_urls":["https://cdn.example.com/out.mp4"],"duration":6,"resolution":"1080p"}`),
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
	if video.Seconds != "6" {
		t.Fatalf("Seconds = %q", video.Seconds)
	}
	if video.Size != "1080p" {
		t.Fatalf("Size = %q", video.Size)
	}
}
