# interactions API の形（調査メモ）

Lyria を呼ぶ `interactions` エンドポイントの、リクエストとレスポンスの形をまとめたものです。
公式ドキュメントの例が最小限で、[Music generation](https://ai.google.dev/gemini-api/docs/music-generation)
だけでは形が確定しません。**このリポジトリの実装はここに書いてある型に合わせています。**

出典は 3 つです。

- **公式 Python SDK `google-genai` 2.22.0** — `google/genai/_gaos/types/interactions/` にある
  型定義（Speakeasy による生成コード）。以下の抜粋は原文のままです
- **公式 REST リファレンス** — [Interactions API](https://ai.google.dev/api/interactions-api)。
  リクエストのトップレベル、`AudioResponseFormat`、`generation_config` の中身、`status` の
  列挙が確認できます。SDK の型定義と食い違いはありませんでした
- **実測** — 2026-09-07 に `lyria-3.5` を実際に呼んだ結果

なお [Interactions overview](https://ai.google.dev/gemini-api/docs/interactions-overview) は
概念の説明で、JSON の形は載っていません。形が要るときはリファレンスのほうを見てください。

WAV が拒否される件は README の[凍結の理由](../README.md#-凍結の理由-blocked-upstream)にあります。
ここは「API がどういう形か」だけを扱います。

---

## エンドポイント

```
POST https://generativelanguage.googleapis.com/v1beta/interactions
x-goog-api-key: $GEMINI_API_KEY
Content-Type: application/json
```

`v1` は `lyria-3.5` を知らず（`Model 'lyria-3.5' not found. Did you mean 'lyria-3-pro-preview'?`）、
`v1alpha` は廃止済み（`API version v1alpha is deprecated. Please use v1 or v1beta.`）なので、
`v1beta` が唯一の経路です。

**モデル名は URL ではなくリクエスト本文に載ります。** `generateContent`
（`/models/{model}:generateContent`）との一番大きな違いです。

このキーで見えた Lyria は 3 つでした。

```
models/lyria-3.5              Lyria 3.5
models/lyria-3-pro-preview    Lyria 3 Pro Preview
models/lyria-3-clip-preview   Lyria 3 Clip Preview
```

---

## リクエスト

### CreateModelInteractionParam

`_gaos/types/interactions/createmodelinteraction.py` より抜粋（必須は `input` と `model` だけ）。

```python
class CreateModelInteractionParam(TypedDict):
    r"""Parameters for creating model interactions"""

    input: InteractionsInputParam
    model: Model
    background: NotRequired[bool]
    environment: NotRequired[CreateModelInteractionEnvironmentParam]
    generation_config: NotRequired[GenerationConfigParam]
    labels: NotRequired[Dict[str, str]]
    previous_interaction_id: NotRequired[str]
    response_format: NotRequired[CreateModelInteractionResponseFormatParam]
    response_mime_type: NotRequired[str]
    r"""The mime type of the response. This is required if response_format is set."""
    response_modalities: NotRequired[List[ResponseModality]]
    safety_settings: NotRequired[List[SafetySettingParam]]
    service_tier: NotRequired[ServiceTier]
    store: NotRequired[bool]
    stream: NotRequired[bool]
    system_instruction: NotRequired[str]
    tools: NotRequired[List[ToolParam]]
    webhook_config: NotRequired[WebhookConfigParam]
```

トップレベルの `response_mime_type` と `response_modalities` は **deprecated** で、しかも
**構造化出力（JSON スキーマ）用**です。音声の形式指定とは別系統で、`response_format` と
組で使うと `responseFormat must be set when responseMimeType is set` になります。音声の
形式を決めるのは次の `response_format.mime_type` のほうです。

### input — 文字列でもブロック配列でもよい

```python
InteractionsInputParam = Union[ContentParam, List[StepParam], List[ContentParam], str]
```

添付が無ければ文字列をそのまま渡せます。添付があるときはブロックの配列にします。

```python
class TextContentParam(TypedDict):
    text: str
    annotations: NotRequired[List[AnnotationParam]]
    type: Literal["text"]

class ImageContentParam(TypedDict):
    data: NotRequired[Union[str, Base64FileInput]]   # base64 文字列
    mime_type: NotRequired[ImageContentMimeType]
    resolution: NotRequired[MediaResolution]
    type: Literal["image"]
    uri: NotRequired[str]
```

ブロックの種類は `text` / `document` / `image` / `audio` / `video` です（`content.py` の
`_CONTENT_VARIANTS`）。このライブラリは Lyria の用途で確認できた `text` と `image` だけを送ります。

### response_format — ここが音声の形式指定

```python
AudioResponseFormatMimeType = Literal[
    "audio/mp3", "audio/ogg_opus", "audio/l16", "audio/wav", "audio/alaw", "audio/mulaw"
]
AudioResponseFormatDelivery = Literal["inline", "uri"]

class AudioResponseFormatParam(TypedDict):
    r"""Configuration for audio output format."""
    bit_rate: NotRequired[int]      # 圧縮形式（MP3, Opus）のみ
    delivery: NotRequired[AudioResponseFormatDelivery]
    mime_type: NotRequired[AudioResponseFormatMimeType]
    sample_rate: NotRequired[int]   # Hz
    type: Literal["audio"]
```

単体でもリストでも渡せます。

```python
CreateModelInteractionResponseFormat = Union[ResponseFormat, List[ResponseFormat]]
```

`ResponseFormat` は `audio` / `image` / `text` / `video` の Union で、`type` が判別子です。

### generation_config — generateContent のものとは別物

```python
class GenerationConfigParam(TypedDict):
    r"""Configuration parameters for model interactions."""
    image_config: NotRequired[ImageConfigParam]
    max_output_tokens: NotRequired[int]
    seed: NotRequired[int]
    speech_config: NotRequired[SpeechConfigUnionParam]
    stop_sequences: NotRequired[List[str]]
    thinking_level: NotRequired[ThinkingLevel]
    thinking_summaries: NotRequired[ThinkingSummaries]
    tool_choice: NotRequired[ToolChoiceParam]
    transcription_config: NotRequired[TranscriptionConfigParam]
    video_config: NotRequired[VideoConfigParam]
```

**温度・TopP・TopK・responseMimeType はありません。** `generateContent` の
`GenerationConfig` を写すと落ちるので、`gemini.GenerateOptions` からここへ持ってこられるのは
`Seed` / `MaxOutputTokens` / `StopSequences` だけです。

---

## レスポンス

実測（`lyria-3.5`、`response_format: {"type":"audio"}`）。base64 は省略しています。

```json
{
  "id": "v1_Chcy...",
  "object": "interaction",
  "model": "lyria-3.5",
  "status": "completed",
  "created": "2026-09-06T17:20:24Z",
  "updated": "2026-09-06T17:20:24Z",
  "service_tier": "standard",
  "usage": {
    "total_tokens": 2138,
    "total_input_tokens": 6,
    "total_output_tokens": 2132,
    "total_cached_tokens": 0,
    "total_thought_tokens": 0,
    "total_tool_use_tokens": 0,
    "raw_prompt_token": 7468,
    "input_tokens_by_modality": [{"modality": "text", "tokens": 6}]
  },
  "steps": [
    {"type": "model_output", "content": [{"type": "text", "text": "[[A0]]\n[[B1]]\n[[C2]]..."}]},
    {"type": "model_output", "content": [{"type": "audio", "mime_type": "audio/mpeg", "data": "<base64>"}]}
  ]
}
```

`status` の列挙は次のとおりです（REST リファレンスより）。

```
in_progress, requires_action, completed, failed, cancelled, incomplete, budget_exceeded, queued
```

**`in_progress` と `queued` は失敗ではありません。** リクエストに `background: true` を付けたときに
返る「まだ終わっていない」状態で、`previous_interaction_id` で追う流れになります。
`budget_exceeded` も、再試行して直らない点で他の失敗とは扱いが違います。

このリポジトリの実装は `background` を送らない（同期実行のみ）ため、**`completed` 以外を
まとめて空レスポンス扱い**にしています（`gemini.ErrEmptyResponse` と `ErrIncomplete` の
両方で判定でき、`ResponseError.Status` に状態が入ります）。上の区別は実装していません。凍結中で実機の確認が
できず、確かめずに分岐だけ増やしても正しさを保証できないためです。非同期実行を使う日には
ここを分けてください。

読むときに効く点が 3 つあります。

- **テキストと音声は別の `step` に入ります**（テキストが先）。1 つの step の中を見るだけでは
  両方を拾えません
- `steps[].type` は `model_output` 以外にもあります（`user_input` / `thought` /
  `function_call` など。`step.py` の `StepParam` に一覧）。生成結果は `model_output` だけです
- テキストは Lyria の**譜面**です。区間を `[[A0]]` `[[B1]]` の letter+index で表し、歌唱行は
  `[:]` で始まります（歌詞ありの生成の場合）

音声ブロックの型は次のとおりです。

```python
class AudioContentParam(TypedDict):
    r"""An audio content block."""
    channels: NotRequired[int]
    data: NotRequired[Union[str, Base64FileInput]]
    mime_type: NotRequired[AudioContentMimeType]
    sample_rate: NotRequired[int]
    type: Literal["audio"]
    uri: NotRequired[str]
```

### エラー

`generateContent` と形が違います。**`code` が文字列**です。

```json
{"error": {"message": "Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5",
           "code": "invalid_request"}}
```

---

## 分からないこと

**モデルごとにどの音声形式を出せるのかは、どこにも書かれていません。** REST リファレンスにも
Python SDK にも対応表が無く、`mime_type` の列挙は API 全体で共有された一覧です。実際に叩くまで
分からず、駄目な場合はモデルの手前で
`Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5` として返ります。

つまり「この列挙に載っている = そのモデルで使える」ではありません。README の
[凍結の理由](../README.md#-凍結の理由-blocked-upstream)はこの食い違いの記録です。

---

## 旧モデルの互換シム（参考）

SDK は `lyria-3-pro-preview` と `lyria-3-clip-preview` を「レガシー」として扱い、
レスポンスの `outputs` を `steps` へ読み替えます（`_gaos/google_genai.py` の
`_LEGACY_LYRIA_MODELS`、`interactions/interaction.py` の `_maybe_coerce_outputs`）。

```python
_LEGACY_LYRIA_MODELS = frozenset({'lyria-3-pro-preview', 'lyria-3-clip-preview'})
```

`lyria-3.5` はこれに含まれないので、素直に `steps` が返ります。このリポジトリは
`lyria-3.5` を前提にしており、`outputs` の読み替えは実装していません。旧モデルを
使うなら、そこを足す必要があります。
