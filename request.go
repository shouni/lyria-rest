package lyriarest

import (
	"fmt"
	"math"

	"github.com/shouni/genai-kit/gemini"
)

// wavGenerationConfig は、generationConfig に混ぜて WAV を要求する指定です。
//
// このライブラリの存在理由はこの 1 か所です。genai SDK の GenerateContentConfig には
// 対応するフィールドが無いため、REST を直接叩いています。呼び出し側が渡した
// generationConfig より後に上書きで載せるので、GenerateOptions からこれを打ち消す
// ことはできません（WAV 前提のライブラリなので、選択肢を持たせていません）。
//
// TODO: REST 側のフィールド名を実機で確認して固定する。ここ以外にフォーマット指定は無い。
var wavGenerationConfig = map[string]any{
	"responseMimeType": "audio/wav",
}

// requestBody は generateContent のリクエスト本文です。
type requestBody struct {
	Contents          []content       `json:"contents"`
	SystemInstruction *content        `json:"systemInstruction,omitempty"`
	GenerationConfig  map[string]any  `json:"generationConfig,omitempty"`
	SafetySettings    []safetySetting `json:"safetySettings,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

// part はリクエストとレスポンスで共用します。Thought はレスポンスにだけ現れます。
type part struct {
	Text       string    `json:"text,omitempty"`
	Thought    bool      `json:"thought,omitempty"`
	InlineData *blob     `json:"inlineData,omitempty"`
	FileData   *fileData `json:"fileData,omitempty"`
}

// blob はインライン添付です。Data は encoding/json が base64 で往復させます。
type blob struct {
	MIMEType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

type fileData struct {
	MIMEType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri"`
}

type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// buildRequestBody は、genai-kit の Generate と同じ入力からリクエスト本文を組み立てます。
func buildRequestBody(prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (requestBody, error) {
	parts, err := attachmentParts(prompt, attachments)
	if err != nil {
		return requestBody{}, err
	}

	config, err := generationConfig(opts)
	if err != nil {
		return requestBody{}, err
	}
	for key, value := range wavGenerationConfig {
		config[key] = value
	}

	body := requestBody{
		Contents:         []content{{Role: "user", Parts: parts}},
		GenerationConfig: config,
	}
	if opts.SystemPrompt != "" {
		body.SystemInstruction = &content{Parts: []part{{Text: opts.SystemPrompt}}}
	}
	for _, setting := range opts.SafetySettings {
		if setting == nil {
			continue
		}
		body.SafetySettings = append(body.SafetySettings, safetySetting{
			Category:  string(setting.Category),
			Threshold: string(setting.Threshold),
		})
	}

	return body, nil
}

// attachmentParts は、プロンプトと添付をパート列へ変換します。
//
// 規則は genai-kit の attachmentParts と同じです。送るものが無い添付は読み飛ばし、
// Data と URI の併用と、Data に MIME type が無いものは弾きます。URI 参照の MIME type は
// 省略でき、その場合はサーバー側の判定に委ねます。
func attachmentParts(prompt string, attachments []gemini.Attachment) ([]part, error) {
	parts := make([]part, 0, len(attachments)+1)
	if prompt != "" {
		parts = append(parts, part{Text: prompt})
	}

	for i, attachment := range attachments {
		if attachment.IsEmpty() {
			continue
		}
		if len(attachment.Data) > 0 && attachment.URI != "" {
			return nil, fmt.Errorf("%w: attachments[%d] は Data と URI のどちらか一方だけを設定してください", ErrInvalidAttachment, i)
		}
		if attachment.URI != "" {
			parts = append(parts, part{FileData: &fileData{FileURI: attachment.URI, MIMEType: attachment.MIMEType}})
			continue
		}
		if attachment.MIMEType == "" {
			return nil, fmt.Errorf("%w: attachments[%d] にMIME typeが設定されていません", ErrInvalidAttachment, i)
		}
		parts = append(parts, part{InlineData: &blob{MIMEType: attachment.MIMEType, Data: attachment.Data}})
	}

	if len(parts) == 0 {
		return nil, ErrEmptyParts
	}
	return parts, nil
}

// generationConfig は GenerateOptions のうち generateContent が受け付ける項目を写します。
//
// 音声生成で意味を持つのは Seed だけですが、テキスト向けの項目も落とさずに渡します。
// genai-kit の Generator と同じ入力を受ける以上、渡された値を黙って捨てないためです。
func generationConfig(opts gemini.GenerateOptions) (map[string]any, error) {
	config := map[string]any{}
	if opts.Seed != nil {
		if *opts.Seed < math.MinInt32 || *opts.Seed > math.MaxInt32 {
			return nil, fmt.Errorf("%w: %d", ErrInvalidSeed, *opts.Seed)
		}
		config["seed"] = int32(*opts.Seed)
	}
	if opts.Temperature != nil {
		config["temperature"] = *opts.Temperature
	}
	if opts.TopP != nil {
		config["topP"] = *opts.TopP
	}
	if opts.TopK != nil {
		config["topK"] = *opts.TopK
	}
	if opts.MaxOutputTokens > 0 {
		config["maxOutputTokens"] = opts.MaxOutputTokens
	}
	if len(opts.StopSequences) > 0 {
		config["stopSequences"] = opts.StopSequences
	}
	if opts.ResponseMIMEType != "" {
		config["responseMimeType"] = opts.ResponseMIMEType
	}
	return config, nil
}
