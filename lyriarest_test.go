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

	"github.com/shouni/genai-kit/gemini"
)

// interactionResponse は、Lyria が返す形のレスポンスを組み立てます。
//
// 譜面テキストと音声が別々の step に入り、テキストが先に来るのが実測の並びです。
func interactionResponse(t *testing.T, mimeType string, audio []byte, text string) []byte {
	t.Helper()

	body := map[string]any{
		"status": "completed",
		"model":  "lyria-3.5",
		"steps": []map[string]any{
			{"type": "model_output", "content": []map[string]any{{"type": "text", "text": text}}},
			{"type": "model_output", "content": []map[string]any{
				{"type": "audio", "mime_type": mimeType, "data": base64.StdEncoding.EncodeToString(audio)},
			}},
		},
		"usage": map[string]any{"total_tokens": 15, "total_input_tokens": 10, "total_output_tokens": 5},
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
	body   map[string]any
}

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

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	c, err := New(Config{APIKey: "secret", Endpoint: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func TestNewRequiresAPIKey(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); !errors.Is(err, ErrAPIKeyRequired) {
		t.Errorf("New(Config{}) error = %v, want %v", err, ErrAPIKeyRequired)
	}
}

// TestGenerate は、リクエストの形とレスポンスの変換を通しで検証します。
// genai-kit の lyria が読む場所（Attachments の audio/、Text）が埋まることが要点です。
func TestGenerate(t *testing.T) {
	t.Parallel()

	wav := []byte("RIFF....WAVEfmt ")
	server, rec := newRecordingServer(t, http.StatusOK, interactionResponse(t, "audio/wav", wav, "[[V1]]\n[:] la la"))
	c := newTestClient(t, server)

	seed := int64(7)
	got, err := c.Generate(context.Background(), "models/lyria-3.5", "full prompt", nil, gemini.GenerateOptions{Seed: &seed})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// リクエスト側
	if rec.method != http.MethodPost || rec.path != "/v1beta/interactions" {
		t.Errorf("%s %s, want POST /v1beta/interactions", rec.method, rec.path)
	}
	if rec.header.Get("x-goog-api-key") != "secret" {
		t.Errorf("x-goog-api-key = %q", rec.header.Get("x-goog-api-key"))
	}
	if rec.body["model"] != "lyria-3.5" {
		t.Errorf("model = %v（models/ 接頭辞は落とす）", rec.body["model"])
	}
	if rec.body["input"] != "full prompt" {
		t.Errorf("input = %v, want 文字列", rec.body["input"])
	}
	format, _ := rec.body["response_format"].(map[string]any)
	if format["type"] != "audio" || format["mime_type"] != wavMIMEType {
		t.Errorf("response_format = %v, want type=audio mime_type=%s", format, wavMIMEType)
	}
	config, _ := rec.body["generation_config"].(map[string]any)
	if config["seed"] != float64(7) {
		t.Errorf("seed = %v", config["seed"])
	}

	// レスポンス側
	if got.Text != "[[V1]]\n[:] la la" {
		t.Errorf("Text = %q", got.Text)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "audio/wav" || string(got.Attachments[0].Data) != string(wav) {
		t.Errorf("Attachments = %+v", got.Attachments)
	}
	if len(got.Audios) != 1 {
		t.Errorf("Audios = %d, want 1", len(got.Audios))
	}
	if got.Usage == nil || got.Usage.TotalTokenCount != 15 {
		t.Errorf("Usage = %+v", got.Usage)
	}
}

// TestGenerateReportsRejectedWAV は、いま実際に返る失敗を再現します。
//
// API のバリデーションは audio/wav を通し、その先のモデルが弾きます。凍結の理由が
// このエラーなので、呼び出し側がメッセージまで辿り着けることを保証します。
func TestGenerateReportsRejectedWAV(t *testing.T) {
	t.Parallel()

	body := []byte(`{"error":{"message":"Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5","code":"invalid_request"}}`)
	server, _ := newRecordingServer(t, http.StatusBadRequest, body)
	c := newTestClient(t, server)

	_, err := c.Generate(context.Background(), "lyria-3.5", "p", nil, gemini.GenerateOptions{})

	if !errors.Is(err, ErrHTTP) {
		t.Fatalf("error = %v, want %v", err, ErrHTTP)
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %T, want *HTTPError", err)
	}
	if httpErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d", httpErr.StatusCode)
	}
	if httpErr.Message != "Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5" {
		t.Errorf("Message = %q, want API のメッセージそのもの", httpErr.Message)
	}
}

// TestGenerateRejectsIncompleteInteraction は、completed 以外の状態を空レスポンス扱いに
// することを検証します。genai-kit のセンチネルで分類できるようにしています。
func TestGenerateRejectsIncompleteInteraction(t *testing.T) {
	t.Parallel()

	server, _ := newRecordingServer(t, http.StatusOK, []byte(`{"status":"failed","steps":[]}`))
	c := newTestClient(t, server)

	_, err := c.Generate(context.Background(), "lyria-3.5", "p", nil, gemini.GenerateOptions{})

	if !errors.Is(err, gemini.ErrEmptyResponse) {
		t.Fatalf("error = %v, want %v", err, gemini.ErrEmptyResponse)
	}
	var respErr *ResponseError
	if !errors.As(err, &respErr) || respErr.Status != "failed" {
		t.Errorf("error = %v, want ResponseError{Status: failed}", err)
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
