package common

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyVideoModelRoutingMatchesRuleAndMutatesRequest(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "sora-2",
		ChannelMeta: &ChannelMeta{
			ModelRoutingRules: `{
				"version": 1,
				"rules": [
					{
						"name": "sora 8s landscape",
						"enabled": true,
						"logic": "AND",
						"conditions": [
							{"path": "derived.original_model", "mode": "full", "value": "sora-2"},
							{"path": "derived.duration", "mode": "full", "value": 8},
							{"path": "derived.aspect_ratio", "mode": "full", "value": "16:9"}
						],
						"target_model": "sora-2-8s-16x9",
						"operations": [
							{"path": "request.metadata.variant", "mode": "delete"}
						]
					}
				]
			}`,
		},
	}
	req := TaskSubmitReq{
		Model:       "sora-2",
		Duration:    8,
		DurationSet: true,
		Size:        "1920x1080",
		Metadata: map[string]interface{}{
			"variant": "pro",
		},
	}

	applied, nextReq, err := ApplyVideoModelRouting(info, req)
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "sora-2-8s-16x9", nextReq.Model)
	require.NotContains(t, nextReq.Metadata, "variant")
	require.Equal(t, "sora-2-8s-16x9", info.UpstreamModelName)
	require.True(t, info.IsModelMapped)
}

func TestApplyVideoModelRoutingSupportsPassMissingKeyDefaults(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "sora-2",
		ChannelMeta: &ChannelMeta{
			ModelRoutingRules: `{
				"version": 1,
				"rules": [
					{
						"name": "sora default",
						"enabled": true,
						"logic": "AND",
						"conditions": [
							{"path": "derived.original_model", "mode": "full", "value": "sora-2"},
							{"path": "derived.duration", "mode": "full", "value": 12, "pass_missing_key": true},
							{"path": "derived.aspect_ratio", "mode": "full", "value": "16:9", "pass_missing_key": true}
						],
						"target_model": "sora-2-12s-16x9"
					}
				]
			}`,
		},
	}

	applied, nextReq, err := ApplyVideoModelRouting(info, TaskSubmitReq{Model: "sora-2"})
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, "sora-2-12s-16x9", nextReq.Model)
}

func TestApplyVideoModelRoutingRejectsHeaderOperations(t *testing.T) {
	info := &RelayInfo{
		OriginModelName: "veo",
		ChannelMeta: &ChannelMeta{
			ModelRoutingRules: `{
				"version": 1,
				"rules": [
					{
						"enabled": true,
						"conditions": [
							{"path": "derived.original_model", "mode": "full", "value": "veo"}
						],
						"target_model": "firefly-veo31-fast-8s-16x9-1080p",
						"operations": [
							{"path": "Authorization", "mode": "set_header", "value": "Bearer x"}
						]
					}
				]
			}`,
		},
	}

	_, _, err := ApplyVideoModelRouting(info, TaskSubmitReq{Model: "veo"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not supported")
}

func TestApplyTaskParamOverrideWithRelayInfoPreservesZeroValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("task_request", TaskSubmitReq{
		Model: "veo",
	})

	info := &RelayInfo{
		ChannelMeta: &ChannelMeta{
			ParamOverride: map[string]interface{}{
				"operations": []map[string]interface{}{
					{
						"path":  "model",
						"mode":  "set",
						"value": "firefly-veo31-fast-8s-16x9-1080p",
					},
					{
						"path":  "fps",
						"mode":  "set",
						"value": 0,
					},
					{
						"path":  "metadata.generate_audio",
						"mode":  "set",
						"value": false,
					},
				},
			},
		},
	}

	err := ApplyTaskParamOverrideWithRelayInfo(c, info)
	require.NoError(t, err)
	nextReq, err := GetTaskRequest(c)
	require.NoError(t, err)
	require.Equal(t, "firefly-veo31-fast-8s-16x9-1080p", nextReq.Model)
	require.Equal(t, "firefly-veo31-fast-8s-16x9-1080p", info.UpstreamModelName)
	require.NotNil(t, nextReq.Fps)
	require.Equal(t, 0, *nextReq.Fps)
	require.Equal(t, false, nextReq.Metadata["generate_audio"])
}
