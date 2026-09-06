# 🎧 Lyria REST

[![Status](https://img.shields.io/badge/Status-Blocked%20upstream-lightgrey)](#-凍結の理由-blocked-upstream)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/lyria-rest)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/lyria-rest.svg)](https://pkg.go.dev/github.com/shouni/lyria-rest)

## 🚀 概要 (About) - Lyria を interactions で直接呼び、WAV を要求します。保存も作詞もしません

**Lyria REST** は、音楽生成モデル **Lyria** を Gemini API の `interactions` エンドポイントで
**直接呼び、音声を WAV で要求する**ための Go ライブラリです。返すのは音声バイト列と、モデルが
返すテキストだけで、**保存先は決めません**。歌詞や楽曲レシピを作る工程も持ちません。

> [!WARNING]
> **現在このライブラリは動きません。** Lyria が WAV の要求を受け付けないためで、こちらの
> 実装の問題ではありません。呼び出すと必ず HTTP 400 になります。
> 経緯と、再開できるようになったかの判定方法は[凍結の理由](#-凍結の理由-blocked-upstream)にあります。
>
> 音楽生成の既定の入口は [genai-kit](https://github.com/shouni/genai-kit) の `lyria` です。

シグネチャ・フィールド・エラーの一覧は
[pkg.go.dev](https://pkg.go.dev/github.com/shouni/lyria-rest) にあります。ここに書くのは、
godoc を読んでも気付けないことだけです。

---

## 🧊 凍結の理由 (Blocked upstream)

**WAV は SDK の制約ではなく、モデルが出力していません。** 2026-09-07 に実機で確認しました。

`interactions` の `response_format` には出力形式の口があり、API のバリデーションは
`audio/wav` を正しい値として受け付けます（公式 Python SDK `google-genai` の
`AudioResponseFormat.mime_type` に列挙されています）。ところが、その先のモデルが弾きます。

```
lyria-3.5             → Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5
lyria-3-pro-preview   → 同上
lyria-3-clip-preview  → 同上
```

`audio/l16`・`audio/ogg_opus` も同じで、**既定であるはずの `audio/mp3` を明示しても拒否されます**。
形式指定そのものを受け付けず、常に既定の MP3（`mime_type: audio/mpeg`）を返す状態です。

試して駄目だった経路は次のとおりです。同じ調査を繰り返さないために残します。

| 試したこと | 結果 |
|---|---|
| `generateContent` + `generationConfig.responseFormat.audio.mimeType: AUDIO_WAV` | HTTP 400 |
| `generateContent` + `responseMimeType: audio/wav` | 400（`text/plain`・`application/json` 等のみ許可） |
| `interactions` + `response_format.mime_type: audio/wav` | モデルが拒否（3 モデルとも） |
| 同上をリスト形式 `[{...}]` で | 同じ |
| `interactions` + トップレベル `response_mime_type` | 構造化出力用の別系統（`responseFormat must be set when responseMimeType is set`） |
| `interactions` + `delivery: "uri"` | `Audio delivery mode is not supported` |
| `Accept: audio/wav` ヘッダ / `?alt=media` | 無視され、MP3 が返る |

**公式ドキュメントとは食い違っています。** [Music generation](https://ai.google.dev/gemini-api/docs/music-generation)
の "Select output format" は「`response_format` を設定すれば WAV を要求できる」と書いていますが、
続く curl の例は `{"type": "audio"}` だけで WAV を示すフィールドがなく、そのまま実行すると MP3 が
返ります。ドキュメントが実装より先行しているものと思われます。

API の形そのもの（リクエスト・レスポンスの構造、Python SDK の型定義）は
[docs/interactions-api.md](docs/interactions-api.md) にまとめてあります。

### 再開できるか調べる方法

この 1 コマンドで判定できます。WAV が返るようになったら、このライブラリは**何も変えずに動きます**。

```sh
curl -s -X POST "https://generativelanguage.googleapis.com/v1beta/interactions" \
  -H "x-goog-api-key: $GEMINI_API_KEY" -H "Content-Type: application/json" \
  -d '{"model":"lyria-3.5","input":"piano","response_format":{"type":"audio","mime_type":"audio/wav"}}'
```

---

## ✨ 提供機能 (Features)

* **genai-kit の `gemini.Generator` をそのまま満たします**: これが設計の中心です。genai-kit の
  `lyria.New` に `lyria.WithAudioGenerator(client)` で渡すと、**音声生成だけがこちらを通り**、
  作詞・作曲・`Track`・呼び出しガード・プロンプト構築は genai-kit のものがそのまま動きます。
  SDK に戻すときはオプションを外すだけです。genai-kit を import するのは型を共有するためで、
  SDK の呼び出しは含みません。
* **WAV 前提です**: 出力フォーマットを選ばせる口は置きません。フォーマットを選びたいのではなく
  **WAV が欲しい**からこのライブラリがあるので、選択肢を持つと存在理由がぼやけます。MP3 で
  よいなら genai-kit を使うほうが得です（リトライ・認証・型の面倒を SDK が見ます）。
* **バックエンドは Gemini API だけです**: `interactions` は Gemini API のエンドポイントで、
  Lyria もそちらにしか無いためです。`v1` は `lyria-3.5` を知らず、`v1alpha` は廃止済みなので、
  `v1beta` が唯一の経路です。
* **失敗の分類は genai-kit のセンチネルで判定できます**: 空のレスポンスは
  `gemini.ErrEmptyResponse` で `errors.Is` が通ります。API が 2xx 以外を返した場合は
  `HTTPError`（`StatusCode` と API のメッセージ付き、`errors.Is(err, ErrHTTP)`）です。
* **リトライを持ちません**: SDK 内蔵のリトライは通りません。再試行の判断は呼び出し側で
  行ってください。発射間隔・上限時間・重複排除が要る場合は、genai-kit の `callguard` で包み、
  テキスト生成と 1 つのガードを共有する形でワークフロー層に置いてください。
* **保存も後処理もしません**: 返すのはバイト列です。GCS への書き出しは
  [go-remote-io](https://github.com/shouni/go-remote-io)、WAV の無劣化結合や読みの正規化は
  [audio](https://github.com/shouni/audio) が持ちます。

---

## 🚦 使い方 (Usage)

```sh
go get github.com/shouni/lyria-rest
```

genai-kit の `gemini.Generator` と同じ呼び方です。

```go
client, err := lyriarest.New(lyriarest.Config{APIKey: os.Getenv("GEMINI_API_KEY")})
if err != nil {
	log.Fatal(err)
}

resp, err := client.Generate(ctx, "lyria-3.5",
	"Title: 'Light Me Up'. Euphoric festival EDM pop, 126 BPM, F minor.",
	nil, gemini.GenerateOptions{})
if err != nil {
	log.Fatal(err) // 現在はここで HTTP 400（凍結の理由を参照）
}

// resp.Audios[0] が音声、resp.Text にはモデルが返す譜面テキストが入ります。
```

genai-kit のワークフローに差し込むなら、`lyria.New` のオプションに渡します。
作詞・作曲は従来どおり SDK のクライアントが担い、音声だけがこちらを通ります。

```go
workflow, err := lyria.New(sdkClient, textPrompts, audioPrompts,
	lyria.WithGeminiModel("gemini-3.8-flash"),
	lyria.WithLyriaModel("lyria-3.5"),
	lyria.WithAudioGenerator(client), // ← SDK が WAV に対応したら、この 1 行を外す
)
```

---

## 🤝 依存関係 (Dependencies)

- [genai-kit](https://github.com/shouni/genai-kit) - `gemini.Generator` / `Attachment` / `GenerateOptions` / `Response` の型を共有するため。SDK の呼び出しは含みません

---

## 📜 ライセンス (License)

MIT License. 詳細は [LICENSE](LICENSE) を参照してください。
