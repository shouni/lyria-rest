package lyriarest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/auth/credentials"

	"github.com/shouni/genai-kit/gemini"
)

// skipWithoutGCPCredentials は、Application Default Credentials が使えない環境
// （CI ランナーなど）でテストをスキップします。New の Vertex AI 経路だけがこれを要します。
func skipWithoutGCPCredentials(t *testing.T) {
	t.Helper()

	if _, err := credentials.DetectDefault(&credentials.DetectOptions{Scopes: []string{cloudPlatformScope}}); err != nil {
		t.Skipf("ADC が見つからないため、このテストをスキップします: %v", err)
	}
}

// audioResponseJSON は、音声 1 件とテキストを返す generateContent のレスポンスを組み立てます。
func audioResponseJSON(t *testing.T, mimeType string, audio []byte, text string) []byte {
	t.Helper()

	body := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{"parts": []map[string]any{
				{"text": text},
				{"inlineData": map[string]any{"mimeType": mimeType, "data": base64.StdEncoding.EncodeToString(audio)}},
			}},
			"finishReason": "STOP",
		}},
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

// recordedRequest は、テストサーバーが受け取ったリクエストの要点です。
type recordedRequest struct {
	method string
	path   string
	header http.Header
	body   requestBody
}

// newRecordingServer は、受け取ったリクエストを記録して固定のレスポンスを返すサーバーを作ります。
func newRecordingServer(t *testing.T, status int, response []byte) (*httptest.Server, *recordedRequest) {
	t.Helper()

	rec := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.header = r.Method, r.URL.Path, r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &rec.body)
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	t.Cleanup(server.Close)
	return server, rec
}

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); !errors.Is(err, ErrConfigRequired) {
		t.Errorf("New(Config{}) error = %v, want %v", err, ErrConfigRequired)
	}
}

// TestNewVertexAIDetectsCredentialsUpFront は、Vertex AI の設定では New の時点で ADC を
// 検出することを検証します。呼び出し時まで先送りすると、認証の設定漏れに最初の生成で気付きます。
func TestNewVertexAIDetectsCredentialsUpFront(t *testing.T) {
	skipWithoutGCPCredentials(t)

	c, err := New(Config{ProjectID: "p", LocationID: "us-central1"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.token == nil {
		t.Error("token = nil, want ADC から取るクロージャ")
	}
}

// TestGenerateGeminiAPI は、API キー経路の URL・ヘッダ・本文と、レスポンスの変換を
// 通しで検証します。genai-kit の lyria が読む場所（Attachments の audio/、Text）が埋まることが要点です。
func TestGenerateGeminiAPI(t *testing.T) {
	t.Parallel()

	wav := []byte("RIFF....WAVEfmt ")
	server, rec := newRecordingServer(t, http.StatusOK, audioResponseJSON(t, "audio/wav", wav, "[[V1]]\n[:] la la"))
	c, err := New(Config{APIKey: "secret", Endpoint: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	seed := int64(7)
	got, err := c.Generate(context.Background(), "models/lyria-3.5", "full prompt",
		[]gemini.Attachment{{MIMEType: "image/png", Data: []byte("cover")}},
		gemini.GenerateOptions{Seed: &seed})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// リクエスト側
	if rec.method != http.MethodPost {
		t.Errorf("method = %s", rec.method)
	}
	if rec.path != "/v1beta/models/lyria-3.5:generateContent" {
		t.Errorf("path = %s（models/ 接頭辞は落とす）", rec.path)
	}
	if rec.header.Get("x-goog-api-key") != "secret" {
		t.Errorf("x-goog-api-key = %q", rec.header.Get("x-goog-api-key"))
	}
	if rec.header.Get("Authorization") != "" {
		t.Error("API キー経路で Authorization ヘッダが付いています")
	}
	if len(rec.body.Contents) != 1 || len(rec.body.Contents[0].Parts) != 2 {
		t.Fatalf("Contents = %+v, want プロンプト + 画像の 2 パート", rec.body.Contents)
	}
	if rec.body.GenerationConfig["seed"] != float64(7) {
		t.Errorf("seed = %v", rec.body.GenerationConfig["seed"])
	}
	for key := range wavGenerationConfig {
		if _, ok := rec.body.GenerationConfig[key]; !ok {
			t.Errorf("generationConfig に WAV 指定 %q が載っていません", key)
		}
	}

	// レスポンス側
	if got.Text != "[[V1]]\n[:] la la" {
		t.Errorf("Text = %q", got.Text)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "audio/wav" || string(got.Attachments[0].Data) != string(wav) {
		t.Errorf("Attachments = %+v", got.Attachments)
	}
}

// TestGenerateVertexAI は、Vertex AI 経路の URL と Bearer ヘッダを検証します。
// ADC は使わず、トークン取得だけを差し替えます。
func TestGenerateVertexAI(t *testing.T) {
	t.Parallel()

	server, rec := newRecordingServer(t, http.StatusOK, audioResponseJSON(t, "audio/wav", []byte{1}, ""))
	c := &Client{
		cfg:   Config{ProjectID: "proj", LocationID: "us-central1", Endpoint: server.URL},
		http:  server.Client(),
		token: func(context.Context) (string, error) { return "tok", nil },
	}

	if _, err := c.Generate(context.Background(), "lyria-3.5", "p", nil, gemini.GenerateOptions{}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if rec.path != "/v1/projects/proj/locations/us-central1/publishers/google/models/lyria-3.5:generateContent" {
		t.Errorf("path = %s", rec.path)
	}
	if rec.header.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization = %q", rec.header.Get("Authorization"))
	}
	if rec.header.Get("x-goog-api-key") != "" {
		t.Error("Vertex AI 経路で x-goog-api-key が付いています")
	}
}

// TestGenerateReportsHTTPFailure は、2xx 以外がステータス付きの HTTPError になることを検証します。
func TestGenerateReportsHTTPFailure(t *testing.T) {
	t.Parallel()

	server, _ := newRecordingServer(t, http.StatusTooManyRequests, []byte(`{"error":{"message":"quota"}}`))
	c, err := New(Config{APIKey: "k", Endpoint: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = c.Generate(context.Background(), "lyria-3.5", "p", nil, gemini.GenerateOptions{})

	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("error = %v, want %v", err, ErrHTTP)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("error = %v, want HTTPError{429}", err)
	}
}

func TestGenerateRejectsEmptyModel(t *testing.T) {
	t.Parallel()

	c, err := New(Config{APIKey: "k"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = c.Generate(context.Background(), "models/", "p", nil, gemini.GenerateOptions{})

	if !errors.Is(err, ErrEmptyModelName) {
		t.Errorf("error = %v, want %v", err, ErrEmptyModelName)
	}
}
