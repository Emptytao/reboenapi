# new-api 标准 API 参考文档

本文面向下游开发者，描述当前这套 `new-api` 实例可用于二次开发的标准接口、请求格式、返回格式、流式调用方式和错误处理方式。

这是一份“调用文档”，不是后台运维文档。

如果你需要查看当前实例已经接入了哪些模型、哪些视频渠道、哪些模型映射规则，请同时参考：

- [当前 new-api API 与模型梳理](./current-newapi-api-guide.zh-CN.md)

## 1. 基础信息

### 1.1 Base URL

请将下文所有路径拼接到你的 `new-api` 服务地址后使用，例如：

```text
https://your-domain.example.com
```

### 1.2 认证方式

所有标准接口默认使用 Bearer Token：

```http
Authorization: Bearer <YOUR_NEWAPI_TOKEN>
```

示例：

```bash
curl https://your-domain.example.com/v1/models \
  -H "Authorization: Bearer sk-xxxx"
```

### 1.3 内容类型

常规 JSON 接口：

```http
Content-Type: application/json
```

文件上传接口：

- `multipart/form-data`
- 主要用于图片编辑、音频转写、音频翻译、部分视频输入

## 2. 调用约定

### 2.1 动态模型

当前实例实际能用哪些模型，是动态的，取决于：

- 你使用的 token
- token 所属分组
- 后台已启用的渠道
- 渠道填写的模型列表

因此，对接前请先调用：

- `GET /v1/models`

### 2.2 OpenAI 兼容为主

这套 `new-api` 以 OpenAI 风格接口为主，常用入口包括：

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/images/generations`
- `/v1/embeddings`
- `/v1/audio/*`
- `/v1/videos`

此外还提供：

- `Claude` 风格：`/v1/messages`
- `Gemini Native` 风格：`/v1beta/models/{model}:generateContent`

### 2.3 流式返回

以下接口通常支持流式：

- `/v1/chat/completions`
- `/v1/responses`
- 某些上游支持的音频或兼容扩展接口

开启方式：

```json
{
  "stream": true
}
```

流式返回格式为：

- `Content-Type: text/event-stream`
- 数据块格式为 `data: {...}\n\n`
- 结束时通常返回 `data: [DONE]`

### 2.4 错误返回

标准错误体通常为：

```json
{
  "error": {
    "message": "The model 'xxx' does not exist",
    "type": "invalid_request_error",
    "param": "model",
    "code": "model_not_found"
  }
}
```

常见错误字段：

- `message`：错误说明
- `type`：错误类别
- `param`：出错参数
- `code`：错误码

说明：

- 大多数中继接口会返回标准 HTTP 错误状态码
- 少数查询型接口可能返回 `200`，但 body 中携带 `error`

因此客户端建议同时判断：

- HTTP 状态码
- 响应体是否存在 `error`

## 3. 模型查询

## 3.1 `GET /v1/models`

查询当前 token 可用模型列表。

请求：

```bash
curl https://your-domain.example.com/v1/models \
  -H "Authorization: Bearer sk-xxxx"
```

典型响应：

```json
{
  "success": true,
  "data": [
    {
      "id": "gpt-4o-mini",
      "object": "model",
      "created": 1626777600,
      "owned_by": "openai",
      "supported_endpoint_types": [
        "chat_completions",
        "responses"
      ]
    }
  ],
  "object": "list"
}
```

字段说明：

- `id`：模型名
- `owned_by`：优先匹配到的渠道归属名
- `supported_endpoint_types`：该模型适合调用的接口类型

## 3.2 `GET /v1/models/{model}`

查询单个模型信息。

请求：

```bash
curl https://your-domain.example.com/v1/models/gpt-4o-mini \
  -H "Authorization: Bearer sk-xxxx"
```

## 4. 文本对话

## 4.1 `POST /v1/chat/completions`

最通用的文本、多模态、工具调用接口。

### 请求头

```http
Authorization: Bearer <YOUR_NEWAPI_TOKEN>
Content-Type: application/json
```

### 常用请求字段

- `model`：模型名，必填
- `messages`：消息数组，必填
- `stream`：是否流式
- `stream_options`：流式附加选项
- `temperature`
- `top_p`
- `max_tokens`
- `max_completion_tokens`
- `tools`
- `tool_choice`
- `response_format`
- `reasoning_effort`
- `metadata`

### 基础示例

```bash
curl https://your-domain.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {
        "role": "user",
        "content": "请用一句话介绍你自己"
      }
    ]
  }'
```

### 典型返回

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "gpt-4o-mini",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "我是一个 AI 助手。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 9,
    "total_tokens": 21
  }
}
```

### 流式示例

```bash
curl https://your-domain.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "gpt-4o-mini",
    "stream": true,
    "stream_options": {
      "include_usage": true
    },
    "messages": [
      {
        "role": "user",
        "content": "写一首四句短诗"
      }
    ]
  }'
```

### 多模态消息格式

`messages[].content` 支持字符串，也支持内容数组。

文本 + 图片示例：

```json
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "描述这张图片"
        },
        {
          "type": "image_url",
          "image_url": {
            "url": "https://example.com/cat.jpg",
            "detail": "high"
          }
        }
      ]
    }
  ]
}
```

当前代码可识别的内容类型：

- `text`
- `image_url`
- `input_audio`
- `file`
- `video_url`

注意：

- 是否真正可用取决于具体上游模型是否支持
- `new-api` 负责兼容解析，不保证所有渠道都支持所有多模态类型

### Tool Calling 示例

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "user",
      "content": "查询北京天气"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "查询天气",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {
              "type": "string"
            }
          },
          "required": [
            "city"
          ]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

### 结构化输出示例

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {
      "role": "user",
      "content": "提取联系人信息"
    }
  ],
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "contact_info",
      "schema": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string"
          },
          "phone": {
            "type": "string"
          }
        },
        "required": [
          "name",
          "phone"
        ]
      }
    }
  }
}
```

## 4.2 `POST /v1/completions`

兼容旧版 Completion 风格接口。

常用字段：

- `model`
- `prompt`
- `max_tokens`
- `temperature`
- `stream`

建议新项目优先使用：

- `/v1/chat/completions`
- `/v1/responses`

## 5. Responses API

## 5.1 `POST /v1/responses`

这是面向新式 OpenAI 调用方式的统一推理接口，适合：

- 多轮推理
- 工具调用
- 多模态输入
- 结构化输出

### 常用请求字段

- `model`：必填
- `input`：必填
- `stream`
- `max_output_tokens`
- `temperature`
- `reasoning`
- `tools`
- `tool_choice`
- `previous_response_id`
- `metadata`

### 基础示例

```bash
curl https://your-domain.example.com/v1/responses \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1-mini",
    "input": "请总结下面这段话"
  }'
```

### 数组输入示例

```json
{
  "model": "gpt-4.1-mini",
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "描述这张图"
        },
        {
          "type": "input_image",
          "image_url": "https://example.com/sample.png"
        }
      ]
    }
  ]
}
```

### 支持的 `input` 内容类型

- `input_text`
- `input_image`
- `input_file`

### 流式示例

```bash
curl https://your-domain.example.com/v1/responses \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "gpt-4.1-mini",
    "stream": true,
    "input": "给我一份三点总结"
  }'
```

## 5.2 `POST /v1/responses/compact`

这是压缩型 Responses 接口，适合对 payload 体积敏感的客户端。

建议：

- 普通项目优先用 `/v1/responses`
- 仅在你明确需要更轻量的返回体时使用 `/v1/responses/compact`

## 6. 图像接口

## 6.1 `POST /v1/images/generations`

文生图接口。

### 常用字段

- `model`
- `prompt`
- `n`
- `size`
- `quality`
- `response_format`
- `background`
- `moderation`
- `output_format`
- `watermark`

### 基础示例

```bash
curl https://your-domain.example.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "A clean product photo of a silver mechanical keyboard",
    "size": "1024x1024"
  }'
```

### 典型返回

```json
{
  "created": 1710000000,
  "data": [
    {
      "url": "https://example.com/generated.png",
      "b64_json": "",
      "revised_prompt": ""
    }
  ]
}
```

### 尺寸注意事项

对于内置兼容校验：

- `dall-e-2` / `dall-e`
  - 允许：`256x256`、`512x512`、`1024x1024`
- `dall-e-3`
  - 允许：`1024x1024`、`1024x1792`、`1792x1024`

## 6.2 `POST /v1/images/edits`

图像编辑接口。

### 内容类型

```http
Content-Type: multipart/form-data
```

### 常用表单字段

- `model`
- `prompt`
- `image`
- `image[]`
- `mask`
- `size`
- `quality`
- `watermark`

### 单图编辑示例

```bash
curl https://your-domain.example.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxxx" \
  -F "model=gpt-image-1" \
  -F "prompt=把背景改成纯白色" \
  -F "image=@/path/to/input.png"
```

### 多图编辑示例

```bash
curl https://your-domain.example.com/v1/images/edits \
  -H "Authorization: Bearer sk-xxxx" \
  -F "model=gpt-image-1" \
  -F "prompt=融合成一张电商主图" \
  -F "image[]=@/path/to/a.png" \
  -F "image[]=@/path/to/b.png"
```

## 6.3 `POST /v1/edits`

兼容型图像编辑入口，通常与 `/v1/images/edits` 语义接近。

## 7. Embedding

## 7.1 `POST /v1/embeddings`

文本向量化接口。

### 常用字段

- `model`
- `input`
- `encoding_format`
- `dimensions`

### 基础示例

```bash
curl https://your-domain.example.com/v1/embeddings \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": [
      "第一段文本",
      "第二段文本"
    ]
  }'
```

### 典型返回

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.01, 0.02]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}
```

## 7.2 `POST /v1/engines/{model}/embeddings`

旧版兼容入口，建议新项目统一使用：

- `/v1/embeddings`

## 8. 音频接口

## 8.1 `POST /v1/audio/transcriptions`

音频转文字。

### 内容类型

```http
Content-Type: multipart/form-data
```

### 常用表单字段

- `file`：必填
- `model`：必填
- `response_format`
- `language`
- 其他上游专属参数

### 示例

```bash
curl https://your-domain.example.com/v1/audio/transcriptions \
  -H "Authorization: Bearer sk-xxxx" \
  -F "model=whisper-1" \
  -F "file=@/path/to/audio.mp3" \
  -F "response_format=json"
```

### 返回

常见返回格式：

```json
{
  "text": "这是转写后的内容"
}
```

如果上游支持详细格式，也可能返回 verbose JSON。

## 8.2 `POST /v1/audio/translations`

音频翻译接口，调用方式与转写基本一致，也是 `multipart/form-data`。

示例：

```bash
curl https://your-domain.example.com/v1/audio/translations \
  -H "Authorization: Bearer sk-xxxx" \
  -F "model=whisper-1" \
  -F "file=@/path/to/audio.mp3"
```

## 8.3 `POST /v1/audio/speech`

文本转语音接口。

### 请求体字段

- `model`
- `input`
- `voice`
- `instructions`
- `response_format`
- `speed`

### 示例

```bash
curl https://your-domain.example.com/v1/audio/speech \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  --output speech.mp3 \
  -d '{
    "model": "gpt-4o-mini-tts",
    "input": "欢迎使用我们的 API",
    "voice": "alloy",
    "response_format": "mp3"
  }'
```

返回通常为音频二进制流，例如：

- `audio/mpeg`
- `audio/wav`
- `audio/ogg`

## 9. Rerank

## 9.1 `POST /v1/rerank`

重排序接口。

### 请求字段

- `model`
- `query`
- `documents`
- `top_n`
- `return_documents`
- `max_chunk_per_doc`
- `overlap_tokens`

### 示例

```bash
curl https://your-domain.example.com/v1/rerank \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "rerank-v1",
    "query": "适合做新手 Go 教程的文章",
    "documents": [
      "Go 并发编程入门",
      "高阶 PostgreSQL 调优",
      "前端动画设计原则"
    ],
    "top_n": 2
  }'
```

### 典型返回

```json
{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.98
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "total_tokens": 20
  }
}
```

## 10. 视频接口

## 10.1 `POST /v1/videos`

统一异步视频生成接口，推荐新项目优先使用。

### 常用字段

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

### 基础文生视频示例

```bash
curl https://your-domain.example.com/v1/videos \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sora-2",
    "prompt": "A cinematic drone shot over a futuristic city at sunrise",
    "size": "1920x1080",
    "duration": 8
  }'
```

### 图生视频示例

```bash
curl https://your-domain.example.com/v1/videos \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "veo",
    "prompt": "让人物自然转头并向前走",
    "images": [
      "https://example.com/frame1.png"
    ],
    "duration": 8,
    "size": "1080x1920"
  }'
```

### 首尾帧视频示例

```json
{
  "model": "sora-2",
  "prompt": "从静止站立过渡到转身离开",
  "images": [
    "https://example.com/start.png",
    "https://example.com/end.png"
  ],
  "duration": 8,
  "size": "1080x1920"
}
```

### 提交响应

通常返回：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "sora-2",
  "status": "queued",
  "progress": 0,
  "created_at": 1710000000
}
```

## 10.2 `GET /v1/videos/{task_id}`

查询视频任务状态。

示例：

```bash
curl https://your-domain.example.com/v1/videos/task_xxx \
  -H "Authorization: Bearer sk-xxxx"
```

典型完成态响应：

```json
{
  "id": "task_xxx",
  "task_id": "task_xxx",
  "object": "video",
  "model": "sora-2",
  "status": "completed",
  "progress": 100,
  "created_at": 1710000000,
  "completed_at": 1710000030,
  "seconds": "8",
  "size": "1920x1080",
  "metadata": {
    "url": "https://cdn.example.com/out.mp4"
  }
}
```

### 状态值

- `queued`
- `in_progress`
- `completed`
- `failed`

## 10.3 `GET /v1/videos/{task_id}/content`

代理下载视频结果。

示例：

```bash
curl -L https://your-domain.example.com/v1/videos/task_xxx/content \
  -H "Authorization: Bearer sk-xxxx" \
  -o out.mp4
```

## 10.4 `POST /v1/videos/{video_id}/remix`

对已有视频任务执行 remix。

注意：

- 是否支持取决于具体视频渠道
- 不是所有视频模型都支持 remix

## 10.5 兼容旧视频接口

仍然可用，但新项目建议统一走 `/v1/videos`：

- `POST /v1/video/generations`
- `GET /v1/video/generations/{task_id}`

## 11. Claude 兼容接口

## 11.1 `POST /v1/messages`

提供 Claude 风格对接入口，适合：

- 你已经在使用 Anthropic SDK
- 或你已有 Claude 风格消息格式

说明：

- 请求头可能还需要客户端自己的 Anthropic 兼容头
- 但在 `new-api` 层面，仍由你的 Bearer Token 做鉴权

## 12. Gemini Native 兼容接口

## 12.1 `POST /v1beta/models/{model}:generateContent`

Gemini 原生格式入口。

适合：

- 已经基于 Gemini Native 协议开发的客户端

普通业务如果没有强需求，建议优先用：

- `/v1/chat/completions`
- `/v1/responses`

## 13. 开发建议

### 13.1 先查模型再发请求

推荐每个接入方都在初始化时调用：

- `GET /v1/models`

用来确认：

- 模型是否存在
- 模型是否属于当前 token
- 模型大致支持哪些 endpoint

### 13.2 客户端要同时判断状态码和错误体

建议判断逻辑：

1. 如果 HTTP 状态码不是 `2xx`，直接视为失败
2. 如果响应体中存在 `error`，也视为失败
3. 对视频任务类接口，还要检查任务状态是否为 `failed`

### 13.3 流式客户端要处理 `[DONE]`

对于 `/v1/chat/completions` 和 `/v1/responses` 的流式调用：

- 逐块读取 `data:`
- 忽略空行
- 读到 `[DONE]` 时结束

### 13.4 文件上传接口要用 `multipart/form-data`

包括：

- `/v1/images/edits`
- `/v1/audio/transcriptions`
- `/v1/audio/translations`

### 13.5 视频接口优先统一用 `/v1/videos`

原因：

- 路径统一
- 兼容多个上游渠道
- 便于后续模型替换和参数适配

## 14. 最小调用清单

如果你是第一次接入这套 `new-api`，最小调用顺序建议是：

1. `GET /v1/models`
2. `POST /v1/chat/completions`
3. 如需新式推理：`POST /v1/responses`
4. 如需图片：`POST /v1/images/generations`
5. 如需音频：`POST /v1/audio/speech` 或 `/v1/audio/transcriptions`
6. 如需视频：`POST /v1/videos` + `GET /v1/videos/{task_id}`

## 15. 相关文档

- [当前 new-api API 与模型梳理](./current-newapi-api-guide.zh-CN.md)
- `docs/openapi/relay.json`
- `router/relay-router.go`
- `router/video-router.go`
