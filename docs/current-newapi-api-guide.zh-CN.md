# 当前 new-api API 与模型梳理

本文基于当前仓库代码整理，重点说明这套 `new-api` 现在对外暴露了哪些 API、哪些模型是动态的、哪些视频模型已经在代码里明确接入。

适用范围：

- 标准 OpenAI 兼容接口
- Claude/Gemini 兼容接口
- 异步视频任务接口
- 本次新增的 `HKCOPP` 与 `SpottedFrog(斑点蛙)` 统一视频适配器

不适用范围：

- 你线上数据库里已经配置了哪些具体聊天模型
- 某个 token 当前被授权了哪些模型

这两类运行时信息是动态的，必须通过接口实时查看。

## 1. 先说结论

这套 `new-api` 目前的模型暴露分成两类：

1. 动态模型
   主要是聊天、推理、图片、音频、Embedding 模型。它们是否可见，取决于：
   - 你后台配置了哪些渠道
   - 渠道 `models` 字段填了什么
   - 用户组 / token 分组
   - token 的模型白名单
   - 是否启用了未配置倍率模型

2. 静态内置模型
   主要是部分任务型渠道，尤其是视频任务适配器。它们的模型名直接写在代码里，接口文档可以稳定整理出来。

## 2. 标准对外 API

认证方式默认都是：

```http
Authorization: Bearer <YOUR_NEWAPI_TOKEN>
```

### 2.1 模型查询

- `GET /v1/models`
- `GET /v1/models/{model}`
- `GET /v1beta/models`
- `GET /v1beta/openai/models`

用途：

- 查询当前 token 实际可用模型
- 这是你查看“当前实例到底开放了哪些聊天/图像/音频模型”的最准入口

示例：

```bash
curl -sS https://your-domain/v1/models \
  -H "Authorization: Bearer sk-xxxx"
```

### 2.2 聊天与推理

- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/responses`
- `POST /v1/responses/compact`
- `POST /v1/messages`  `Claude` 风格
- `GET /v1/realtime`  `OpenAI Realtime WebSocket`

说明：

- `/v1/chat/completions` 是最通用入口
- `/v1/responses` 适合工具调用、结构化输出等新式 OpenAI 风格
- `/v1/messages` 对接 Claude 风格客户端

### 2.3 图像

- `POST /v1/images/generations`
- `POST /v1/images/edits`
- `POST /v1/edits`

### 2.4 Embedding

- `POST /v1/embeddings`
- `POST /v1/engines/{model}/embeddings`

### 2.5 音频

- `POST /v1/audio/transcriptions`
- `POST /v1/audio/translations`
- `POST /v1/audio/speech`

### 2.6 其他

- `POST /v1/rerank`
- `POST /v1/moderations`
- `POST /v1beta/models/{model}:generateContent`  `Gemini Native`

## 3. 视频接口

当前仓库里已经有两套视频入口，建议优先使用 OpenAI 风格接口。

### 3.1 OpenAI 风格视频接口

- `POST /v1/videos`
- `GET /v1/videos/{task_id}`
- `GET /v1/videos/{task_id}/content`
- `POST /v1/videos/{video_id}/remix`

说明：

- 这是现在最推荐的统一视频入口
- 创建任务后返回的是公开任务 ID，格式通常是 `task_xxx`
- 不会把上游真实任务 ID 暴露给下游
- 任务完成后，结果视频地址通常会出现在返回体的 `metadata.url`

### 3.2 兼容旧版视频接口

- `POST /v1/video/generations`
- `GET /v1/video/generations/{task_id}`

### 3.3 Kling 官方风格兼容入口

- `POST /kling/v1/videos/text2video`
- `POST /kling/v1/videos/image2video`
- `GET /kling/v1/videos/text2video/{task_id}`
- `GET /kling/v1/videos/image2video/{task_id}`

### 3.4 即梦官方风格兼容入口

- `POST /jimeng/`

## 4. 标准视频请求字段

当前视频任务标准请求结构已经统一支持这些字段：

- `model`
- `prompt`
- `image`
- `images`
- `mode`
- `size`
- `duration`
- `seconds`
- `width`
- `height`
- `fps`
- `seed`
- `n`
- `response_format`
- `input_reference`
- `metadata`

说明：

- 如果同时传了 `width` 和 `height`，系统会自动归一化为 `size=WxH`
- `duration` / `seconds` 都可用于表达时长
- `metadata` 用于透传上游专属参数
- `fps`、`seed`、`n`、`width`、`height` 在代码里已经改成可保留显式 `0` 的可选字段

通用示例：

```bash
curl https://your-domain/v1/videos \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sora-2",
    "prompt": "A cinematic panda walking through neon rain",
    "width": 1920,
    "height": 1080,
    "duration": 8,
    "metadata": {
      "variant": "pro"
    }
  }'
```

查询任务：

```bash
curl https://your-domain/v1/videos/task_xxx \
  -H "Authorization: Bearer sk-xxxx"
```

下载或代理结果：

```bash
curl -L https://your-domain/v1/videos/task_xxx/content \
  -H "Authorization: Bearer sk-xxxx" \
  -o out.mp4
```

## 5. 当前代码中已明确接入的视频模型

下面这些模型名是代码里已经写死并对外暴露的，不依赖数据库模型表。

### 5.1 Sora 渠道

- `sora-2`
- `sora-2-pro`

### 5.2 Kling 渠道

- `kling-v1`
- `kling-v1-6`
- `kling-v2-master`

### 5.3 即梦渠道

- `jimeng_vgfm_t2v_l20`

### 5.4 阿里万相视频渠道

- `wan2.5-i2v-preview`
- `wan2.2-i2v-flash`
- `wan2.2-i2v-plus`
- `wanx2.1-i2v-plus`
- `wanx2.1-i2v-turbo`

### 5.5 豆包视频渠道

- `doubao-seedance-1-0-pro-250528`
- `doubao-seedance-1-0-lite-t2v`
- `doubao-seedance-1-0-lite-i2v`
- `doubao-seedance-1-5-pro-251215`
- `doubao-seedance-2-0-260128`
- `doubao-seedance-2-0-fast-260128`

### 5.6 海螺视频渠道

- `MiniMax-Hailuo-2.3`
- `MiniMax-Hailuo-2.3-Fast`
- `MiniMax-Hailuo-02`
- `T2V-01-Director`
- `T2V-01`
- `I2V-01-Director`
- `I2V-01-live`
- `I2V-01`
- `S2V-01`

### 5.7 Vidu 渠道

- `viduq2`
- `viduq1`
- `vidu2.0`
- `vidu1.5`

### 5.8 Gemini Veo 渠道

- `veo-3.0-generate-001`
- `veo-3.0-fast-generate-001`
- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`

### 5.9 Vertex Veo 渠道

- `veo-3.0-generate-001`
- `veo-3.0-fast-generate-001`
- `veo-3.1-generate-preview`
- `veo-3.1-fast-generate-preview`

### 5.10 HKCOPP 渠道

渠道类型：

- `58`
- 渠道名：`HKCOPP`
- 默认 Base URL：`https://api.hkcopp.online/openapi/v1`

当前对外模型：

- `seedance-2`
- `seedance-2-fast`
- `seedance-2-fast-beta`
- `seedance-2-beta`
- `seedance-2-fast-beta-face`
- `seedance-2-beta-face`
- `seedance-2-official`
- `seedance-2-official2`
- `seedance-2-official2-fast`
- `seedance-2-intl`
- `seedance-2-intl-fast`
- `seedance-2-intl-xl`
- `seedance-2-intl-xl-fast`
- `kling-v3`
- `kling-v3-pro`
- `vidu-q3-pro`
- `veo-3.1`

适配规则：

- 下游仍走 `/v1/videos`
- 上游提交接口：`POST /generations`
- 上游查询接口：`GET /generations/{task_id}`
- `mode` 推导规则：
  - 无图：`text_to_video`
  - 1 张图：`image_to_video`
  - 2 张图：`first_last_frame`
  - 3 张及以上：`multi_ref`
- `size` 会自动映射成 `ratio`
- 完成后视频地址优先取 `result_url` 或 `result_urls[0]`

### 5.11 SpottedFrog / 斑点蛙统一视频渠道

渠道类型：

- `59`
- 渠道名：`SpottedFrog`
- 经典前端显示名：`斑点蛙`
- 默认 Base URL：`https://api.hellobabygo.com`

当前对外模型：

- `sora-2`
- `omni_flash`
- `grok-imagine-video`
- `veo`

这是一个“统一对外模型名”的适配器，真正上游型号会按参数自动映射。

#### `sora-2` 映射规则

- 时长只允许 `4 / 8 / 12`
- 默认时长 `12`
- 横屏映射：`sora-2-{duration}s-16x9`
- 竖屏映射：`sora-2-{duration}s-9x16`
- 如果 `metadata.variant=pro`：
  - 只允许 `12s`
  - 映射到 `sora2-pro-12s-16x9` 或 `sora2-pro-12s-9x16`

示例：

- `model=sora-2, duration=8, width=1920, height=1080`
- 实际上游：`sora-2-8s-16x9`

#### `omni_flash` 映射规则

- 对外模型名保持 `omni_flash`
- 通过 `size` 或 `width/height` 识别横竖屏
- 参考图最多按文档支持 7 张

#### `grok-imagine-video` 映射规则

- 对外模型名保持 `grok-imagine-video`
- 文生、图生都走统一 `/v1/videos` 异步任务

#### `veo` 映射规则

- 根据 `metadata.speed=fast|standard`
- 根据是否带 `reference_images`
- 根据时长、横竖屏、分辨率
- 自动映射为 `firefly-veo31-*` 对应型号

示例：

- `model=veo, size=1920x1080, duration=8, metadata.speed=fast`
- 实际上游通常映射为：`firefly-veo31-fast-8s-16x9-1080p`

## 6. 如何查看“你当前线上实例真正开放了哪些模型”

因为聊天、图片、音频模型大多是动态的，所以实际以接口返回为准。

### 6.1 查当前 token 的可用模型

```bash
curl -sS https://your-domain/v1/models \
  -H "Authorization: Bearer sk-xxxx"
```

### 6.2 后台渠道里建议这样配视频模型

如果你要新建 `HKCOPP`：

- 类型：`HKCOPP`
- Base URL：`https://api.hkcopp.online/openapi/v1`
- Key：直接填 API Key 原文
- Models：填上面那串 HKCOPP 模型列表

如果你要新建 `SpottedFrog`：

- 类型：`斑点蛙` 或 `SpottedFrog`
- Base URL：`https://api.hellobabygo.com`
- Key：直接填 API Key 原文
- Models：`sora-2,omni_flash,grok-imagine-video,veo`

## 7. 仓库里已有的原始文档入口

如果你后面还要继续扩文档，建议优先看这几个文件：

- `docs/openapi/relay.json`
- `router/relay-router.go`
- `router/video-router.go`
- `controller/swag_video.go`
- `controller/model.go`
- `relay/common/relay_info.go`

## 8. 一句话总结

这套 `new-api` 现在已经具备：

- 完整的 OpenAI 兼容文本 / 图片 / 音频 / embedding / responses 接口
- 统一的异步视频接口 `/v1/videos`
- 多个任务型视频渠道
- 两个新增的视频适配器：
  - `HKCOPP`
  - `SpottedFrog(斑点蛙)`

如果后面要继续整理成“给外部开发者看的正式 API 文档”，最合理的下一步是把这份文档再拆成：

- 标准 OpenAI 兼容接口文档
- 视频接口文档
- 渠道接入文档
- 模型映射文档
