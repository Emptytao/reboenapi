# new-api 用户接入文档

本文面向接入你这套 `new-api` 中转站的下游开发者，说明如何获取令牌、查询模型、调用聊天/图片/音频/视频接口，以及如何用 OpenAI SDK 直接接入。

这是一份“模型调用文档”，不包含后台管理接口。

## 1. 接入概览

你的中转站对下游主要暴露以下几类接口：

- `GET /v1/models`：查询当前令牌可用模型
- `POST /v1/chat/completions`：最常用的聊天/多模态接口
- `POST /v1/responses`：OpenAI 新式响应接口
- `POST /v1/images/generations`：图片生成
- `POST /v1/images/edits`：图片编辑
- `POST /v1/embeddings`：向量接口
- `POST /v1/audio/transcriptions`：语音转文字
- `POST /v1/audio/translations`：语音翻译
- `POST /v1/audio/speech`：文本转语音
- `POST /v1/videos`：异步视频生成任务

如果你的客户端本身是 OpenAI 兼容客户端，通常只需要改两项：

- `base_url`
- `api_key`

## 2. Base URL 与认证

假设你的中转站地址是：

```text
https://your-domain.example.com
```

调用时使用 Bearer Token 认证：

```http
Authorization: Bearer sk-xxxx
```

说明：

- `sk-xxxx` 是你在中转站里签发给用户的 API Key
- 不是后台登录密码
- 不是 `/api/user/token` 这种控制台访问令牌

常规 JSON 请求头：

```http
Content-Type: application/json
```

文件上传场景请使用：

```http
Content-Type: multipart/form-data
```

## 3. 推荐接入流程

建议下游按这个顺序接入：

1. 先拿到 API Key
2. 调用 `GET /v1/models` 获取当前可用模型
3. 优先用 `POST /v1/chat/completions` 接聊天能力
4. 需要结构化推理时再接 `POST /v1/responses`
5. 需要图片、音频、视频时按对应接口扩展

## 4. 查询可用模型

### 4.1 获取模型列表

```bash
curl https://your-domain.example.com/v1/models \
  -H "Authorization: Bearer sk-xxxx"
```

示例返回：

```json
{
  "success": true,
  "data": [
    {
      "id": "gpt-4o-mini",
      "object": "model",
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

说明：

- 一个用户实际可见哪些模型，是动态的
- 不同 token、不同分组、不同渠道配置，看到的模型可能不同
- 所以不要把模型列表硬编码死，建议启动时或定时拉一次 `/v1/models`

### 4.2 获取单个模型信息

```bash
curl https://your-domain.example.com/v1/models/gpt-4o-mini \
  -H "Authorization: Bearer sk-xxxx"
```

## 5. 聊天接口

## 5.1 `POST /v1/chat/completions`

这是最通用、兼容性最好的入口，绝大多数应用优先使用它。

常用字段：

- `model`：模型名，必填
- `messages`：消息数组，必填
- `stream`：是否流式
- `temperature`
- `top_p`
- `max_tokens`
- `tools`
- `tool_choice`
- `response_format`
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
        "role": "system",
        "content": "你是一个专业的中文助手"
      },
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
        "content": "我是一个可以帮助你完成问答、写作和开发辅助的 AI 助手。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 18,
    "total_tokens": 43
  }
}
```

### 流式示例

```bash
curl https://your-domain.example.com/v1/chat/completions \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "请分三行介绍杭州"
      }
    ]
  }'
```

流式返回是标准 SSE：

- `Content-Type: text/event-stream`
- 每个数据块格式为 `data: {...}`
- 结束时通常返回 `data: [DONE]`

## 5.2 多模态消息

如果模型支持图文输入，可以直接在 `messages[].content` 中传图片。

示例：

```json
{
  "model": "gpt-4o-mini",
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
            "url": "https://example.com/demo.png"
          }
        }
      ]
    }
  ]
}
```

## 6. Responses 接口

## 6.1 `POST /v1/responses`

如果你的客户端或业务流程本来就是按 OpenAI 新版 Responses API 设计的，可以直接接这个接口。

常见场景：

- 工具调用
- 结构化输出
- 新版推理链路

示例：

```bash
curl https://your-domain.example.com/v1/responses \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "input": "请输出一个三项待办清单"
  }'
```

如果你没有明确依赖 Responses 语义，建议优先使用 `/v1/chat/completions`。

## 6.2 `POST /v1/responses/compact`

这是更轻量的响应体版本，适合你明确需要更紧凑返回结构的场景。

## 7. 图片接口

## 7.1 `POST /v1/images/generations`

用于文生图。

示例：

```bash
curl https://your-domain.example.com/v1/images/generations \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "一只站在咖啡馆门口的橘猫，电影感光影"
  }'
```

常用字段：

- `model`
- `prompt`
- `size`
- `n`
- `response_format`

## 7.2 `POST /v1/images/edits`

用于图片编辑，通常使用 `multipart/form-data` 上传参考图。

常见表单字段：

- `model`
- `prompt`
- `image`
- `mask`
- `size`

## 8. Embedding 接口

## 8.1 `POST /v1/embeddings`

用于文本向量化。

```bash
curl https://your-domain.example.com/v1/embeddings \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "这是一个向量化测试"
  }'
```

## 9. 音频接口

## 9.1 `POST /v1/audio/transcriptions`

语音转文字，使用 `multipart/form-data`。

常见字段：

- `file`
- `model`
- `language`
- `prompt`

## 9.2 `POST /v1/audio/translations`

音频翻译，通常是把非英语音频转为英语文本。

## 9.3 `POST /v1/audio/speech`

文本转语音。

示例：

```bash
curl https://your-domain.example.com/v1/audio/speech \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "tts-1",
    "voice": "alloy",
    "input": "欢迎使用 new-api 中转站"
  }' \
  --output speech.mp3
```

## 10. 视频接口

## 10.1 `POST /v1/videos`

这是推荐的统一视频入口，适合异步视频生成。

支持的典型请求字段：

- `model`
- `prompt`
- `image`
- `images`
- `mode`
- `size`
- `width`
- `height`
- `duration`
- `seconds`
- `fps`
- `seed`
- `n`
- `response_format`
- `metadata`

示例：

```bash
curl https://your-domain.example.com/v1/videos \
  -H "Authorization: Bearer sk-xxxx" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sora-2",
    "prompt": "雨夜中的霓虹街道，一只柴犬穿着风衣慢慢走过",
    "width": 1920,
    "height": 1080,
    "duration": 8
  }'
```

典型返回：

```json
{
  "id": "task_xxx",
  "object": "video",
  "status": "queued",
  "model": "sora-2"
}
```

说明：

- 返回的是中转站公开任务 ID，例如 `task_xxx`
- 不会暴露上游真实任务 ID
- 视频任务通常是异步的，需要继续轮询查询

## 10.2 `GET /v1/videos/{task_id}`

查询任务状态：

```bash
curl https://your-domain.example.com/v1/videos/task_xxx \
  -H "Authorization: Bearer sk-xxxx"
```

状态通常会统一成：

- `queued`
- `in_progress`
- `completed`
- `failed`

任务完成后，通常可在返回体 `metadata.url` 中拿到结果地址。

## 10.3 `GET /v1/videos/{task_id}/content`

直接下载或代理视频内容：

```bash
curl -L https://your-domain.example.com/v1/videos/task_xxx/content \
  -H "Authorization: Bearer sk-xxxx" \
  -o output.mp4
```

## 10.4 当前视频模型可传参数整理

下面这部分是给下游开发者最有用的“参数能力表”。

说明：

- 这些模型都走统一接口：`POST /v1/videos`
- 通用字段都可以传：`model`、`prompt`、`image`、`images`、`size`、`width`、`height`、`duration`、`seconds`、`fps`、`seed`、`n`、`response_format`、`metadata`
- 但不同模型真正会使用哪些字段、支持哪些组合，不一样

### A. `sora-2`

对外模型名：

```text
sora-2
```

推荐可传参数：

- `prompt`
- `duration` 或 `seconds`
- `size` 或 `width + height`
- `images`：可用于图生视频 / 首尾帧视频
- `metadata.variant`

当前建议能力口径：

- 时长：`4s`、`8s`、`12s`
- 画幅：横屏 `16:9`、竖屏 `9:16`
- 推荐分辨率：`720P`
  - 横屏建议：`1280x720`
  - 竖屏建议：`720x1280`

额外说明：

- 如果你要走 Pro，不要传 `sora-2-pro`
- 当前统一写法是：
  - `model: "sora-2"`
  - `metadata.variant: "pro"`
- Pro 只支持：
  - `12s`
  - 横屏 `16:9` 或竖屏 `9:16`

推荐示例：

```json
{
  "model": "sora-2",
  "prompt": "雨夜里的霓虹街道，一只柴犬穿着风衣走过镜头",
  "duration": 8,
  "size": "1280x720"
}
```

Sora Pro 示例：

```json
{
  "model": "sora-2",
  "prompt": "镜头从人物背后推进到侧脸特写",
  "duration": 12,
  "size": "720x1280",
  "metadata": {
    "variant": "pro"
  }
}
```

### B. `omni_flash`

对外模型名：

```text
omni_flash
```

推荐可传参数：

- `prompt`
- `duration` 或 `seconds`
- `size` 或 `width + height`
- `image` / `images`

当前建议能力口径：

- 时长：可传，建议明确传值，不要依赖默认值
- 画幅：横屏 / 竖屏都可通过 `size` 自动识别
- 参考图：最多 `7` 张

推荐示例：

```json
{
  "model": "omni_flash",
  "prompt": "让人物自然抬头并露出微笑",
  "seconds": 10,
  "size": "1280x720",
  "images": [
    "https://example.com/ref1.png"
  ]
}
```

### C. `grok-imagine-video`

对外模型名：

```text
grok-imagine-video
```

推荐可传参数：

- `prompt`
- `duration` 或 `seconds`
- `size` 或 `width + height`
- `image` / `images`

当前建议能力口径：

- 文生视频：支持
- 图生视频：支持
- 横屏 / 竖屏：可通过 `size` 自动识别
- 没有额外强制的本地组合限制

推荐示例：

```json
{
  "model": "grok-imagine-video",
  "prompt": "一个未来城市的航拍镜头，清晨雾气缓慢散开",
  "duration": 8,
  "size": "1280x720"
}
```

### D. `veo`

对外模型名：

```text
veo
```

推荐可传参数：

- `prompt`
- `duration` 或 `seconds`
- `size` 或 `width + height`
- `image` / `images`
- `metadata.speed`
- `metadata.resolution`
- `metadata.reference_images`

当前建议能力口径：

- 默认时长：`8s`
- 推荐时长：优先使用 `8s`
- 画幅：横屏 `16:9`、竖屏 `9:16`
- 推荐分辨率：`1080P`
  - 横屏建议：`1920x1080`
  - 竖屏建议：`1080x1920`
- 速度档位：
  - `metadata.speed = "fast"`
  - `metadata.speed = "standard"`
- 参考图模式：
  - 只要传了 `image` / `images`
  - 或 `metadata.reference_images`
  - 就会优先走参考图 Veo 型号

推荐组合：

- `8s + 16:9 + 1080p + fast`
- `8s + 9:16 + 1080p + fast`
- `8s + 16:9 + 1080p + standard`
- `8s + 9:16 + 1080p + standard`
- `8s + 16:9 + 1080p + reference`
- `8s + 9:16 + 1080p + reference`

Veo Fast 示例：

```json
{
  "model": "veo",
  "prompt": "人物从窗边转身走向门口，镜头稳定跟拍",
  "duration": 8,
  "size": "1920x1080",
  "metadata": {
    "speed": "fast"
  }
}
```

Veo Standard 示例：

```json
{
  "model": "veo",
  "prompt": "人物从窗边转身走向门口，镜头稳定跟拍",
  "duration": 8,
  "size": "1080x1920",
  "metadata": {
    "speed": "standard"
  }
}
```

Veo 参考图示例：

```json
{
  "model": "veo",
  "prompt": "让人物从静止站立过渡到转身离开",
  "duration": 8,
  "size": "1920x1080",
  "images": [
    "https://example.com/ref1.png"
  ],
  "metadata": {
    "speed": "fast",
    "resolution": "1080p"
  }
}
```

## 10.5 最推荐你给用户的“简版参数表”

如果你要把这部分直接发给用户，可以用下面这版：

| 模型 | 推荐时长 | 推荐画幅 | 推荐分辨率 | 特殊参数 |
| --- | --- | --- | --- | --- |
| `sora-2` | `4s / 8s / 12s` | `16:9` / `9:16` | `720P` | `metadata.variant=pro` 代表 Pro，仅支持 `12s` |
| `omni_flash` | 建议显式传 | 横屏 / 竖屏 | 按 `size` 传 | 最多 `7` 张参考图 |
| `grok-imagine-video` | 建议显式传 | 横屏 / 竖屏 | 按 `size` 传 | 无额外特殊参数 |
| `veo` | 推荐 `8s` | `16:9` / `9:16` | 推荐 `1080P` | `metadata.speed=fast/standard`，传图即参考图模式 |

## 11. OpenAI SDK 接入示例

## 11.1 Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxxx",
    base_url="https://your-domain.example.com/v1",
)

resp = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[
        {"role": "user", "content": "请用一句话介绍杭州"}
    ],
)

print(resp.choices[0].message.content)
```

## 11.2 Node.js

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-xxxx",
  baseURL: "https://your-domain.example.com/v1",
});

const resp = await client.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [
    { role: "user", content: "请用一句话介绍杭州" }
  ]
});

console.log(resp.choices[0].message.content);
```

## 11.3 流式示例

### Python

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-xxxx",
    base_url="https://your-domain.example.com/v1",
)

stream = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "分三段介绍西湖"}],
    stream=True,
)

for chunk in stream:
    delta = chunk.choices[0].delta
    if getattr(delta, "content", None):
        print(delta.content, end="")
```

### Node.js

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-xxxx",
  baseURL: "https://your-domain.example.com/v1",
});

const stream = await client.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [
    { role: "user", content: "分三段介绍西湖" }
  ],
  stream: true,
});

for await (const chunk of stream) {
  const text = chunk.choices?.[0]?.delta?.content;
  if (text) process.stdout.write(text);
}
```

## 12. 错误处理

标准错误体通常是：

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

建议客户端同时检查：

- HTTP 状态码
- 响应体中的 `error`

常见错误原因：

- token 无效
- 模型不可用
- 当前分组无权限
- 请求参数格式错误
- 上游超时或限流

## 13. 最佳实践

### 13.1 模型不要写死

始终优先调用：

```text
GET /v1/models
```

### 13.2 聊天优先用 `/v1/chat/completions`

除非你明确需要 Responses 语义，否则优先使用：

```text
POST /v1/chat/completions
```

### 13.3 视频优先用 `/v1/videos`

视频推荐统一使用：

```text
POST /v1/videos
GET /v1/videos/{task_id}
```

### 13.4 做好流式与非流式双兼容

如果你要适配更多模型，建议客户端支持：

- `stream=false`
- `stream=true`

### 13.5 文件接口注意请求格式

以下接口通常不是纯 JSON：

- `/v1/images/edits`
- `/v1/audio/transcriptions`
- `/v1/audio/translations`

请使用 `multipart/form-data`。

## 14. 建议给接入方的最小文档集

如果你要发给外部开发者，最少给这四项就够他们接起来：

1. 服务地址，例如 `https://your-domain.example.com`
2. 认证方式：`Authorization: Bearer sk-xxxx`
3. 模型列表接口：`GET /v1/models`
4. 主要调用接口：
   - `POST /v1/chat/completions`
   - `POST /v1/images/generations`
   - `POST /v1/audio/speech`
   - `POST /v1/videos`

如果你只想给他们一版最简接入说明，建议直接发：

- Base URL
- API Key
- 一个 `GET /v1/models` 示例
- 一个 `POST /v1/chat/completions` 示例
- 一个 OpenAI SDK 示例
