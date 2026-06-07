package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RequestPreviewLog struct {
	Id                    int    `json:"id" gorm:"index:idx_preview_created_at_id,priority:1;index:idx_preview_user_id_id,priority:2"`
	UserId                int    `json:"user_id" gorm:"index;index:idx_preview_user_id_id,priority:1"`
	Username              string `json:"username" gorm:"index;default:''"`
	CreatedAt             int64  `json:"created_at" gorm:"bigint;index:idx_preview_created_at_id,priority:2"`
	ChannelId             int    `json:"channel_id" gorm:"index"`
	ChannelName           string `json:"channel_name" gorm:"-"`
	ChannelType           int    `json:"channel_type" gorm:"index"`
	RequestPath           string `json:"request_path" gorm:"type:varchar(191);index;default:''"`
	RelayMode             string `json:"relay_mode" gorm:"type:varchar(64);index;default:''"`
	OriginModelName       string `json:"origin_model_name" gorm:"type:varchar(191);index;default:''"`
	UpstreamModelName     string `json:"upstream_model_name" gorm:"type:varchar(191);index;default:''"`
	ClientRequestedStream bool   `json:"client_requested_stream"`
	RequestId             string `json:"request_id,omitempty" gorm:"type:varchar(64);index;default:''"`
	Group                 string `json:"group" gorm:"index;default:''"`
	UpstreamURL           string `json:"upstream_url" gorm:"type:text"`
	Payload               string `json:"payload" gorm:"type:text"`
}

type RecordRequestPreviewLogParams struct {
	ChannelId             int    `json:"channel_id"`
	ChannelType           int    `json:"channel_type"`
	RequestPath           string `json:"request_path"`
	RelayMode             string `json:"relay_mode"`
	OriginModelName       string `json:"origin_model_name"`
	UpstreamModelName     string `json:"upstream_model_name"`
	ClientRequestedStream bool   `json:"client_requested_stream"`
	RequestId             string `json:"request_id"`
	Group                 string `json:"group"`
	UpstreamURL           string `json:"upstream_url"`
	Payload               string `json:"payload"`
}

type RequestPreviewLogQueryParams struct {
	ModelName      string
	Username       string
	RequestID      string
	RequestPath    string
	StartTimestamp int64
	EndTimestamp   int64
	ChannelID      int
}

func RecordRequestPreviewLog(c *gin.Context, userId int, params RecordRequestPreviewLogParams) {
	if userId == 0 {
		return
	}
	username := ""
	if c != nil {
		username = c.GetString("username")
	}
	if username == "" {
		username, _ = GetUsernameById(userId, false)
	}
	log := &RequestPreviewLog{
		UserId:                userId,
		Username:              username,
		CreatedAt:             common.GetTimestamp(),
		ChannelId:             params.ChannelId,
		ChannelType:           params.ChannelType,
		RequestPath:           params.RequestPath,
		RelayMode:             params.RelayMode,
		OriginModelName:       params.OriginModelName,
		UpstreamModelName:     params.UpstreamModelName,
		ClientRequestedStream: params.ClientRequestedStream,
		RequestId:             params.RequestId,
		Group:                 params.Group,
		UpstreamURL:           params.UpstreamURL,
		Payload:               params.Payload,
	}
	if err := LOG_DB.Create(log).Error; err != nil {
		if c != nil {
			logger.LogError(c, "failed to record request preview log: "+err.Error())
			return
		}
		common.SysError("failed to record request preview log: " + err.Error())
	}
}

func applyRequestPreviewModelFilter(tx *gorm.DB, modelName string) (*gorm.DB, error) {
	if modelName == "" {
		return tx, nil
	}
	if containsLikeWildcard(modelName) {
		pattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, err
		}
		return tx.Where(
			"(request_preview_logs.origin_model_name LIKE ? ESCAPE '!' OR request_preview_logs.upstream_model_name LIKE ? ESCAPE '!')",
			pattern,
			pattern,
		), nil
	}
	return tx.Where(
		"(request_preview_logs.origin_model_name = ? OR request_preview_logs.upstream_model_name = ?)",
		modelName,
		modelName,
	), nil
}

func containsLikeWildcard(value string) bool {
	return value != "" && (containsRune(value, '%') || containsRune(value, '_'))
}

func containsRune(value string, target rune) bool {
	for _, r := range value {
		if r == target {
			return true
		}
	}
	return false
}

func GetAllRequestPreviewLogs(startIdx int, num int, queryParams RequestPreviewLogQueryParams) (logs []*RequestPreviewLog, total int64, err error) {
	tx := LOG_DB.Model(&RequestPreviewLog{})
	if tx, err = applyRequestPreviewModelFilter(tx, queryParams.ModelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "request_preview_logs.username", queryParams.Username); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "request_preview_logs.request_path", queryParams.RequestPath); err != nil {
		return nil, 0, err
	}
	if queryParams.RequestID != "" {
		tx = tx.Where("request_preview_logs.request_id = ?", queryParams.RequestID)
	}
	if queryParams.StartTimestamp != 0 {
		tx = tx.Where("request_preview_logs.created_at >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		tx = tx.Where("request_preview_logs.created_at <= ?", queryParams.EndTimestamp)
	}
	if queryParams.ChannelID != 0 {
		tx = tx.Where("request_preview_logs.channel_id = ?", queryParams.ChannelID)
	}
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = tx.Order("request_preview_logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	if err = fillRequestPreviewChannelNames(logs); err != nil {
		return logs, total, err
	}
	return logs, total, nil
}

func GetUserRequestPreviewLogs(userId int, startIdx int, num int, queryParams RequestPreviewLogQueryParams) (logs []*RequestPreviewLog, total int64, err error) {
	tx := LOG_DB.Model(&RequestPreviewLog{}).Where("request_preview_logs.user_id = ?", userId)
	if tx, err = applyRequestPreviewModelFilter(tx, queryParams.ModelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "request_preview_logs.request_path", queryParams.RequestPath); err != nil {
		return nil, 0, err
	}
	if queryParams.RequestID != "" {
		tx = tx.Where("request_preview_logs.request_id = ?", queryParams.RequestID)
	}
	if queryParams.StartTimestamp != 0 {
		tx = tx.Where("request_preview_logs.created_at >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		tx = tx.Where("request_preview_logs.created_at <= ?", queryParams.EndTimestamp)
	}
	if queryParams.ChannelID != 0 {
		tx = tx.Where("request_preview_logs.channel_id = ?", queryParams.ChannelID)
	}
	if err = tx.Limit(logSearchCountLimit).Count(&total).Error; err != nil {
		common.SysError("failed to count request preview logs: " + err.Error())
		return nil, 0, errors.New("查询请求预览日志失败")
	}
	if err = tx.Order("request_preview_logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error; err != nil {
		common.SysError("failed to query request preview logs: " + err.Error())
		return nil, 0, errors.New("查询请求预览日志失败")
	}
	if err = fillRequestPreviewChannelNames(logs); err != nil {
		return logs, total, err
	}
	return logs, total, nil
}

func fillRequestPreviewChannelNames(logs []*RequestPreviewLog) error {
	channelIDs := types.NewSet[int]()
	for _, item := range logs {
		if item.ChannelId != 0 {
			channelIDs.Add(item.ChannelId)
		}
	}
	if channelIDs.Len() == 0 {
		return nil
	}
	var channels []struct {
		Id   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if common.MemoryCacheEnabled {
		for _, channelID := range channelIDs.Items() {
			if cacheChannel, err := CacheGetChannel(channelID); err == nil {
				channels = append(channels, struct {
					Id   int    `gorm:"column:id"`
					Name string `gorm:"column:name"`
				}{
					Id:   channelID,
					Name: cacheChannel.Name,
				})
			}
		}
	} else {
		if err := DB.Table("channels").Select("id, name").Where("id IN ?", channelIDs.Items()).Find(&channels).Error; err != nil {
			return fmt.Errorf("query channels failed: %w", err)
		}
	}
	channelMap := make(map[int]string, len(channels))
	for _, item := range channels {
		channelMap[item.Id] = item.Name
	}
	for i := range logs {
		logs[i].ChannelName = channelMap[logs[i].ChannelId]
	}
	return nil
}
