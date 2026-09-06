package lyriarest

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	// geminiAPIHost は Gemini API（API キー方式）のホストです。
	geminiAPIHost = "https://generativelanguage.googleapis.com"
	// geminiAPIVersion は Gemini API のバージョンです。Lyria は v1beta で提供されています。
	geminiAPIVersion = "v1beta"
	// vertexAPIVersion は Vertex AI のバージョンです。
	vertexAPIVersion = "v1"
	// vertexGlobalLocation は、リージョン接頭辞の付かないグローバルエンドポイントを指す LocationID です。
	vertexGlobalLocation = "global"
)

// Config は初期化用の設定です。
//
// genai-kit の gemini.Config と同じ組み立てです。ProjectID と LocationID を渡せば Vertex AI
// （認証は Application Default Credentials）、APIKey を渡せば Gemini API になります。
// 両方を渡すことはできません。
type Config struct {
	ProjectID  string // Vertex AI: Google Cloud Project ID
	LocationID string // Vertex AI: Location（"us-central1" や "global"）
	APIKey     string // Gemini API（Google AI Studio）のキー。ProjectID/LocationID と排他

	// HTTPClient は REST 呼び出しに使う HTTP クライアントです。nil なら http.Client の
	// ゼロ値（タイムアウト無し）を使い、打ち切りは呼び出し側の context にのみ従います。
	//
	// genai-kit と違い、渡したクライアントの認証を付け直す処理はありません。認証はヘッダで
	// 行うため、Transport を差し替えても失われないからです。
	HTTPClient *http.Client

	// Endpoint はベース URL の上書きです。空なら公式ホストを使います。
	// テストや私設プロキシ向けで、通常は設定しません。
	Endpoint string
}

// validate は設定内容が正しいかをチェックします。
//
// 判定の順序と分類は genai-kit の gemini.Config と揃えています。同じ間違いには同じ
// 種類のエラーが返るほうが、両者を差し替えて使う側の分岐が 1 つで済むためです。
func (c Config) validate() error {
	hasVertexField := c.ProjectID != "" || c.LocationID != ""

	if hasVertexField && c.APIKey != "" {
		return ErrExclusiveConfig
	}
	if !hasVertexField {
		if c.APIKey != "" {
			return nil
		}
		return ErrConfigRequired
	}
	if c.ProjectID == "" || c.LocationID == "" {
		return ErrIncompleteVertexConfig
	}
	return nil
}

// usesAPIKey は、Gemini API バックエンドを使う設定かを返します。
func (c Config) usesAPIKey() bool {
	return c.APIKey != ""
}

// baseURL は API のベース URL を返します。
//
// Vertex AI の "global" ロケーションだけはホストにリージョン接頭辞が付きません。
func (c Config) baseURL() string {
	if c.Endpoint != "" {
		return strings.TrimRight(c.Endpoint, "/")
	}
	if c.usesAPIKey() {
		return geminiAPIHost
	}
	if strings.EqualFold(c.LocationID, vertexGlobalLocation) {
		return "https://aiplatform.googleapis.com"
	}
	return fmt.Sprintf("https://%s-aiplatform.googleapis.com", c.LocationID)
}

// generateContentURL は、モデルの generateContent エンドポイントの URL を返します。
func (c Config) generateContentURL(model string) string {
	if c.usesAPIKey() {
		return fmt.Sprintf("%s/%s/models/%s:generateContent", c.baseURL(), geminiAPIVersion, model)
	}
	return fmt.Sprintf("%s/%s/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		c.baseURL(), vertexAPIVersion, c.ProjectID, c.LocationID, model)
}

// normalizeModel は、モデル名の表記ゆれを URL に埋め込める形へ揃えます。
//
// genai SDK は "lyria-3.5" と "models/lyria-3.5" の両方を受け付けるため、差し替え前に
// どちらで書かれていても同じ URL になるようにしています。
func normalizeModel(model string) string {
	return strings.TrimPrefix(strings.TrimSpace(model), "models/")
}
