// Package lyriarest は、Gemini API の Lyria を interactions エンドポイントで直接呼び、
// WAV を要求します。
//
// genai SDK には音声の出力フォーマットを指定する口が無く、既定の MP3 しか受け取れません。
// REST の interactions には response_format.mime_type があるため、その 1 点のために SDK を
// 迂回するのがこのパッケージです。
//
// 2026-09-07 時点で、Lyria のどのモデルもこの指定を受け付けません。API のバリデーションは
// audio/wav を通し、その先のモデルが弾きます。したがって現在このパッケージは常に
// HTTP 400 を返します。凍結の経緯と再開の判定方法は README にあります。
//
// Client は genai-kit の gemini.Generator を満たします。genai-kit の lyria.New には
// lyria.WithAudioGenerator でこの Client を渡せるので、Workflow・Track・呼び出しガード・
// プロンプト構築はすべて genai-kit のものをそのまま使い回せます。戻すときはオプションを
// 外すだけです。genai-kit を import するのは型（Generator / Attachment / GenerateOptions /
// Response）を共有するためで、SDK の呼び出しは含みません。
package lyriarest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/shouni/genai-kit/gemini"
)

const (
	// maxResponseBytes はレスポンス本文の読み込み上限です。
	// 3 分の音声は base64 で数十 MB になるため、余裕を持たせつつ無制限にはしません。
	maxResponseBytes = 256 << 20
	// maxErrorBodyBytes は、エラー本文を解釈できなかったときに残す長さです。
	maxErrorBodyBytes = 4 << 10
)

// Client がパッケージ公開インターフェースを満たすことをコンパイル時に保証します。
// これが崩れると lyria.WithAudioGenerator に渡せなくなります。
var _ gemini.Generator = (*Client)(nil)

// Client は Lyria の interactions クライアントです。
type Client struct {
	cfg  Config
	http *http.Client
	// maxResponseBytes はレスポンス本文の上限です。テストで小さくする以外は定数のままです。
	maxResponseBytes int64
}

// New は提供された設定に基づいてクライアントを作成します。
func New(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{cfg: cfg, http: httpClient, maxResponseBytes: maxResponseBytes}, nil
}

// Generate は、プロンプトと添付から生成を実行します。genai-kit の gemini.Generator と同じ契約です。
//
// リトライは持ちません。SDK 内蔵のリトライは通らないため、必要なら呼び出し側で
// genai-kit の callguard などで包んでください。打ち切りは呼び出し側の context に従います。
func (c *Client) Generate(ctx context.Context, model string, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (*gemini.Response, error) {
	model = normalizeModel(model)
	if model == "" {
		return nil, ErrEmptyModelName
	}

	body, err := buildRequestBody(model, prompt, attachments, opts)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("lyriarest: リクエストの組み立てに失敗しました: %w", err)
	}

	data, err := c.post(ctx, payload)
	if err != nil {
		return nil, err
	}
	return parseResponse(data)
}

// post は interactions エンドポイントへ本文を送り、2xx のレスポンス本文を返します。
//
// 2xx 以外は HTTPError、上限超過は ErrResponseTooLarge になります。API の形の解釈は
// 呼び出し側（buildRequestBody / parseResponse）に置き、ここは HTTP のやり取りだけを持ちます。
func (c *Client) post(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.interactionsURL(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("lyriarest: リクエストの作成に失敗しました: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lyriarest: 呼び出しに失敗しました: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := readLimited(resp.Body, c.maxResponseBytes)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 != 2 {
		code, message := parseAPIError(data, maxErrorBodyBytes)
		return nil, &HTTPError{StatusCode: resp.StatusCode, Code: code, Message: message}
	}
	return data, nil
}

// readLimited は r を最大 limit バイトまで読み、それを超えていれば ErrResponseTooLarge を返します。
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, fmt.Errorf("lyriarest: レスポンスの読み込みに失敗しました: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
