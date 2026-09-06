# 🎧 Lyria REST

[![Status](https://img.shields.io/badge/Status-Active-brightgreen)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/lyria-rest)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/lyria-rest.svg)](https://pkg.go.dev/github.com/shouni/lyria-rest)

## 🚀 概要 (About) - Lyria を REST で直接呼び、WAV で受け取ります。保存も作詞もしません

**Lyria REST** は、音楽生成モデル **Lyria** を **REST（generateContent）で直接呼び、WAV を受け取る**
ための Go ライブラリです。返すのは音声バイト列と、モデルが返すテキストだけで、**保存先は決めません**。
歌詞や楽曲レシピを作る工程も持ちません。

> [!IMPORTANT]
> **これは、genai SDK が出力フォーマットの指定に対応するまでの繋ぎです。既定の入口は
> [genai-kit](https://github.com/shouni/genai-kit) の `lyria` のほうです。**
>
> `google.golang.org/genai` の `GenerateContentConfig` には Lyria の出力フォーマットを指定する
> フィールドが無く、既定のエンコード結果しか受け取れません。WAV が要る用途——無劣化で結合する、
> 後段でマスタリングする、可逆のまま原盤を残す——では、そこが天井になります。REST にはその口が
> あるので、**その 1 点のために SDK を迂回する**のがこのライブラリです。
>
> 逆に言えば、**WAV が要らないならこれを使う理由はありません。** そして **SDK が対応した時点で、
> このリポジトリは役目を終えます。**

シグネチャ・フィールド・エラーの一覧は
[pkg.go.dev](https://pkg.go.dev/github.com/shouni/lyria-rest) にあります。ここに書くのは、
godoc を読んでも気付けないことだけです。

---

## ✨ 提供機能 (Features)

* **genai-kit の `gemini.Generator` をそのまま満たします**: これが設計の中心です。genai-kit の
  `lyria.New` に `lyria.WithAudioGenerator(client)` で渡すと、**音声生成だけがこちらを通り**、
  作詞・作曲・`Track`・呼び出しガード・プロンプト構築は genai-kit のものがそのまま動きます。
  SDK に戻すときはオプションを外すだけです。genai-kit を import するのは型を共有するためで、
  SDK の呼び出しは含みません。
* **WAV 前提です**: 出力フォーマットを選ばせる口は置きません。フォーマットを選びたいのではなく
  **WAV が欲しい**からこのライブラリがあるので、選択肢を持つと存在理由がぼやけます。
  `GenerateOptions` で何を渡しても WAV の指定が勝ちます。
* **`Config` は genai-kit と同じ組み立てです**: `ProjectID` / `LocationID` なら Vertex AI
  （認証は Application Default Credentials）、`APIKey` なら Gemini API。併用はエラーです。
  設定の間違いに対するエラーの分類も genai-kit と揃えてあります。
* **失敗の分類は genai-kit のセンチネルで判定できます**: ブロックは `gemini.ErrBlocked`、
  空レスポンスは `gemini.ErrEmptyResponse` で `errors.Is` が通ります。genai-kit の経路に
  戻しても、呼び出し側の分岐は同じです。
* **リトライを持ちません**: SDK 内蔵のリトライは通りません。2xx 以外は `HTTPError`
  （`StatusCode` 付き、`errors.Is(err, ErrHTTP)`）で返すので、再試行の判断は呼び出し側で
  行ってください。発射間隔・上限時間・重複排除が要る場合は、genai-kit の `callguard` で包み、
  テキスト生成と 1 つのガードを共有する形でワークフロー層に置いてください。
* **保存も後処理もしません**: 返すのはバイト列です。GCS への書き出しは
  [go-remote-io](https://github.com/shouni/go-remote-io)、WAV の無劣化結合や読みの正規化は
  [audio](https://github.com/shouni/audio) が持ちます。

---

## 🔀 genai-kit の `lyria` との使い分け

| | [genai-kit](https://github.com/shouni/genai-kit) の `lyria` | lyria-rest |
| --- | --- | --- |
| 位置づけ | **既定の入口** | SDK が追いつくまでの繋ぎ |
| 呼び出し経路 | genai SDK | REST を直接 |
| 出力 | モデル既定のエンコード | **WAV** |
| 担当範囲 | 作詞 → レシピ → 音声の 3 段ワークフロー | 音声生成 1 回だけ（`gemini.Generator` 1 メソッド） |
| リトライ | SDK 内蔵 | 無し |

**まず genai-kit を見てください。** こちらを選ぶ理由は「WAV が要る」の 1 点だけです。
併用するときも、こちらは genai-kit の `lyria` に差し込んで使います。

---

## 🚦 使い方 (Usage)

```sh
go get github.com/shouni/lyria-rest
```

単体で 1 回呼ぶなら、genai-kit の `gemini.Generator` と同じ呼び方です。

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/shouni/genai-kit/gemini"
	"github.com/shouni/lyria-rest"
)

func main() {
	ctx := context.Background()

	client, err := lyriarest.New(lyriarest.Config{APIKey: os.Getenv("GEMINI_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Generate(ctx, "lyria-3.5",
		"Title: 'Light Me Up'. Euphoric festival EDM pop, 126 BPM, F minor.",
		nil, gemini.GenerateOptions{})
	if err != nil {
		log.Fatal(err)
	}

	// Attachments[0] が WAV です。resp.Text にはモデルが返す譜面テキストが入ります。
	if err := os.WriteFile("track.wav", resp.Audios[0], 0o644); err != nil {
		log.Fatal(err)
	}
}
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
- [cloud.google.com/go/auth](https://pkg.go.dev/cloud.google.com/go/auth) - Vertex AI 経路の Application Default Credentials

---

## 📜 ライセンス (License)

MIT License. 詳細は [LICENSE](LICENSE) を参照してください。
