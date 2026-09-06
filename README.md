# 🎧 Lyria REST

[![Status](https://img.shields.io/badge/Status-WIP-orange)](#)
[![Language](https://img.shields.io/badge/Language-Go-blue)](https://go.dev/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/lyria-rest)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Reference](https://pkg.go.dev/badge/github.com/shouni/lyria-rest.svg)](https://pkg.go.dev/github.com/shouni/lyria-rest)

## 🚀 概要 (About) - Lyria を REST で直接呼び、WAV で受け取ります。保存も作詞もしません

**Lyria REST** は、Vertex AI の音楽生成モデル **Lyria** を **REST で直接呼び、WAV を受け取る**ための
Go ライブラリです。返すのは音声バイト列と、モデルが返すテキストだけで、**保存先は決めません**。
歌詞や楽曲レシピを作る工程も持ちません。

> [!IMPORTANT]
> **これは、SDK が出力フォーマットの指定に対応するまでの繋ぎです。既定の入口は
> [genai-kit](https://github.com/shouni/genai-kit) の `lyria` のほうです。**
>
> `google.golang.org/genai` には Lyria の出力フォーマットを指定する口がなく、既定のエンコード結果しか
> 受け取れません。WAV が要る用途——無劣化で結合する、後段でマスタリングする、可逆のまま原盤を残す——
> では、そこが天井になります。REST の predict エンドポイントには指定する口があるので、**その 1 点のために
> SDK を迂回する**のがこのライブラリです。
>
> 逆に言えば、**WAV が要らないならこれを使う理由はありません。** SDK 経由ならリトライ・認証・型の面倒を
> SDK が見てくれるぶん得です。そして **SDK が対応した時点で、このリポジトリは役目を終えます。**

シグネチャ・フィールド・エラーの一覧は
[pkg.go.dev](https://pkg.go.dev/github.com/shouni/lyria-rest) にあります。ここに書くのは、
godoc を読んでも気付けないことだけです。

---

## ✨ 提供機能 (Features)

* **WAV 前提です**: 出力フォーマットを選ばせる口は置きません。フォーマットを選びたいのではなく
  **WAV が欲しい**からこのライブラリがあるので、選択肢を持つと存在理由がぼやけます。
* **公開 API に SDK の型は現れません**: `google.golang.org/genai` を import しません。REST の
  リクエストとレスポンスはこのライブラリの内側で組み立て、外へは音声バイト列とテキストだけを渡します。
* **認証は Application Default Credentials に委ねます**: 認証情報の配布も、トークンの再取得の
  管理も引き受けません。
* **流量制御を持ちません**: クォータはプロジェクト単位で操作の種類ごとではないため、ライブラリごとに
  独立したレート制限を持たせると合計がクォータを超えます。発射間隔・上限時間・重複排除が要る場合は、
  genai-kit の `callguard` でデコレートし、テキスト生成と 1 つのガードを共有する形でワークフロー層に
  置いてください。
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
| 担当範囲 | 作詞 → レシピ → 音声の 3 段ワークフロー | 音声生成 1 回だけ |

**まず genai-kit を見てください。** こちらを選ぶ理由は「WAV が要る」の 1 点だけです。

---

## 🚦 使い方 (Usage)

```sh
go get github.com/shouni/lyria-rest
```

（実装が入り次第、構築して 1 回呼ぶまでの例をここに置きます。）

---

## 🤝 依存関係 (Dependencies)

（実装が入り次第、記載します。）

---

## 📜 ライセンス (License)

MIT License. 詳細は [LICENSE](LICENSE) を参照してください。
