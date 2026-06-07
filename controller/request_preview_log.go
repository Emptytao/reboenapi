package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

func GetAllRequestPreviewLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelID, _ := strconv.Atoi(c.Query("channel"))
	queryParams := model.RequestPreviewLogQueryParams{
		ModelName:      c.Query("model_name"),
		Username:       c.Query("username"),
		RequestID:      c.Query("request_id"),
		RequestPath:    c.Query("request_path"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      channelID,
	}
	logs, total, err := model.GetAllRequestPreviewLogs(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func GetUserRequestPreviewLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelID, _ := strconv.Atoi(c.Query("channel"))
	queryParams := model.RequestPreviewLogQueryParams{
		ModelName:      c.Query("model_name"),
		RequestID:      c.Query("request_id"),
		RequestPath:    c.Query("request_path"),
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelID:      channelID,
	}
	logs, total, err := model.GetUserRequestPreviewLogs(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

func GetRequestPreviewLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, errors.New("invalid request preview log id"))
		return
	}
	log, err := model.GetRequestPreviewLogByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiError(c, errors.New("request preview log not found"))
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, log)
}
