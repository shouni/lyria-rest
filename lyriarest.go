// Package lyriarest は、Vertex AI / Gemini API の Lyria を REST で直接呼び、WAV を受け取ります。
//
// genai SDK には Lyria の出力フォーマットを指定する口が無く、既定のエンコード結果しか
// 受け取れません。REST の generateContent にはその口があるため、その 1 点のために SDK を
// 迂回するのがこのパッケージです。SDK が対応した時点で役目を終えます。
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

	"cloud.google.com/go/auth/credentials"

	"github.com/shouni/genai-kit/gemini"
)

const (
	// cloudPlatformScope は Vertex AI の呼び出しに使う OAuth スコープです。
	cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"
	// maxResponseBytes はレスポンス本文の読み込み上限です。
	// 3 分の WAV は base64 で数十 MB になるため、余裕を持たせつつ無制限にはしません。
	maxResponseBytes = 256 << 20
	// maxErrorBodyBytes は HTTPError.Body に残すエラー本文の長さです。
	maxErrorBodyBytes = 4 << 10
)

// Client がパッケージ公開インターフェースを満たすことをコンパイル時に保証します。
// これが崩れると lyria.WithAudioGenerator に渡せなくなります。
var _ gemini.Generator = (*Client)(nil)

// Client は Lyria の REST クライアントです。
type Client struct {
	cfg  Config
	http *http.Client
	// token は Vertex AI のアクセストークンを返します。Gemini API では nil です。
	token func(ctx context.Context) (string, error)
}

// New は提供された設定に基づいてクライアントを作成します。
//
// Vertex AI の場合はここで Application Default Credentials を検出します。
// 見つからなければエラーで、呼び出し時まで先送りしません。検出は通信を伴わないため
// context を取りません（トークンの取得は Generate の context で行います）。
func New(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	c := &Client{cfg: cfg, http: httpClient}

	if !cfg.usesAPIKey() {
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{Scopes: []string{cloudPlatformScope}})
		if err != nil {
			return nil, fmt.Errorf("lyriarest: Application Default Credentials の検出に失敗しました: %w", err)
		}
		c.token = func(ctx context.Context) (string, error) {
			token, err := creds.Token(ctx)
			if err != nil {
				return "", err
			}
			return token.Value, nil
		}
	}

	return c, nil
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

	body, err := buildRequestBody(prompt, attachments, opts)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("lyriarest: リクエストの組み立てに失敗しました: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.generateContentURL(model), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("lyriarest: リクエストの作成に失敗しました: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.authorize(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lyriarest: 呼び出しに失敗しました: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("lyriarest: レスポンスの読み込みに失敗しました: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, ErrResponseTooLarge
	}

	if resp.StatusCode/100 != 2 {
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: string(data[:min(len(data), maxErrorBodyBytes)])}
	}

	return parseResponse(data)
}

// authorize は、バックエンドに応じた認証ヘッダを付けます。
func (c *Client) authorize(ctx context.Context, req *http.Request) error {
	if c.cfg.usesAPIKey() {
		req.Header.Set("x-goog-api-key", c.cfg.APIKey)
		return nil
	}
	token, err := c.token(ctx)
	if err != nil {
		return fmt.Errorf("lyriarest: アクセストークンの取得に失敗しました: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}
