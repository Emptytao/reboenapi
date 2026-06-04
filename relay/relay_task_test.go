package relay

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTaskRoutingRulesSkipLegacyModelMappingWhenMatched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("task_request", common.TaskSubmitReq{Model: "sora-2"})
	c.Set("model_mapping", `{"sora-2":"legacy-upstream"}`)

	info := &common.RelayInfo{
		OriginModelName: "sora-2",
		ChannelMeta: &common.ChannelMeta{
			ModelRoutingRules: `{
				"version": 1,
				"rules": [
					{
						"enabled": true,
						"conditions": [
							{"path": "derived.original_model", "mode": "full", "value": "sora-2"}
						],
						"target_model": "routed-upstream"
					}
				]
			}`,
		},
	}
	info.UpstreamModelName = info.OriginModelName

	_, err := common.ApplyVideoModelRoutingWithRelayInfo(c, info)
	require.NoError(t, err)
	if !info.IsModelMapped {
		require.NoError(t, helper.ModelMappedHelper(c, info, nil))
	}
	require.Equal(t, "routed-upstream", info.UpstreamModelName)
}

func TestTaskRoutingRulesFallbackToLegacyModelMappingWhenNoRuleMatched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("task_request", common.TaskSubmitReq{Model: "sora-2"})
	c.Set("model_mapping", `{"sora-2":"legacy-upstream"}`)

	info := &common.RelayInfo{
		OriginModelName: "sora-2",
		ChannelMeta: &common.ChannelMeta{
			ModelRoutingRules: `{
				"version": 1,
				"rules": [
					{
						"enabled": true,
						"conditions": [
							{"path": "derived.original_model", "mode": "full", "value": "veo"}
						],
						"target_model": "routed-upstream"
					}
				]
			}`,
		},
	}
	info.UpstreamModelName = info.OriginModelName

	_, err := common.ApplyVideoModelRoutingWithRelayInfo(c, info)
	require.NoError(t, err)
	if !info.IsModelMapped {
		require.NoError(t, helper.ModelMappedHelper(c, info, nil))
	}
	require.Equal(t, "legacy-upstream", info.UpstreamModelName)
}
