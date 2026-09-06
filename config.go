package lyriarest

import (
	"net/http"
	"strings"
)

const (
	// geminiAPIHost は Gemini API のホストです。
	geminiAPIHost = "https://generativelanguage.googleapis.com"
	// apiVersion は interactions を提供しているバージョンです。
	//
	// v1 は lyria-3.5 を知らず（"Model 'lyria-3.5' not found"）、v1alpha は廃止済みなので、
	// 選択の余地はありません。
	apiVersion = "v1beta"
)

// Config は初期化用の設定です。
//
// Vertex AI の設定はありません。interactions は Gemini API のエンドポイントで、Lyria も
// そちらにしか無いためです（Vertex AI に最新の Lyria が来た日には、ここへ ProjectID /
// LocationID が増えます）。
type Config struct {
	// APIKey は Gemini API のキーです。必須。
	APIKey string

	// HTTPClient は REST 呼び出しに使う HTTP クライアントです。nil なら http.Client の
	// ゼロ値（タイムアウト無し）を使い、打ち切りは呼び出し側の context にのみ従います。
	//
	// 認証はヘッダで行うため、Transport を差し替えても認証は失われません。
	HTTPClient *http.Client

	// Endpoint はベース URL の上書きです。空なら公式ホストを使います。
	// テストや私設プロキシ向けで、通常は設定しません。
	Endpoint string
}

// validate は設定内容が正しいかをチェックします。
func (c Config) validate() error {
	if c.APIKey == "" {
		return ErrAPIKeyRequired
	}
	return nil
}

// interactionsURL は interactions エンドポイントの URL を返します。
//
// モデル名は URL ではなくリクエスト本文で指定します（generateContent とはそこが違います）。
func (c Config) interactionsURL() string {
	base := geminiAPIHost
	if c.Endpoint != "" {
		base = strings.TrimRight(c.Endpoint, "/")
	}
	return base + "/" + apiVersion + "/interactions"
}

// normalizeModel は、モデル名の表記ゆれを本文に載せる形へ揃えます。
//
// genai SDK は "lyria-3.5" と "models/lyria-3.5" の両方を受け付けるため、差し替え前に
// どちらで書かれていても同じ値になるようにしています。
func normalizeModel(model string) string {
	return strings.TrimPrefix(strings.TrimSpace(model), "models/")
}
