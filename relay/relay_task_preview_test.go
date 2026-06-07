package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	appcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayTaskSubmitPreviewModeSkipsTaskCreationFlow(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewBufferString(`{"model":"seedance-2-official","prompt":"hello world","size":"1280x720","duration":8}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("role", appcommon.RoleAdminUser)

	appcommon.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeHKCOPP)
	appcommon.SetContextKey(ctx, constant.ContextKeyChannelId, 58)
	appcommon.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, "http://127.0.0.1:1")
	appcommon.SetContextKey(ctx, constant.ContextKeyChannelKey, "preview-secret")
	appcommon.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	appcommon.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		RequestPreviewModeEnabled: true,
	})

	info, err := relaycommon.GenRelayInfo(ctx, types.RelayFormatTask, nil, nil)
	require.NoError(t, err)

	result, taskErr := RelayTaskSubmit(ctx, info)
	require.Nil(t, taskErr)
	require.Nil(t, result)
	require.True(t, relaychannel.IsRequestPreviewHandled(ctx))
	require.Empty(t, info.PublicTaskID)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp map[string]any
	require.NoError(t, appcommon.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "channel_request_preview", resp["object"])

	channelPayload := resp["channel"].(map[string]any)
	require.EqualValues(t, constant.ChannelTypeHKCOPP, int(channelPayload["type"].(float64)))
}
