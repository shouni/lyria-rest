package lyriarest

import (
	"fmt"
	"math"
	"strings"

	"github.com/shouni/genai-kit/gemini"
)

// wavMIMEType は、音声を WAV で要求する値です。
//
// このライブラリの存在理由はこの 1 か所です。genai SDK には出力フォーマットを指定する口が
// 無いため、REST を直接叩いています。値は API が受け付ける列挙のひとつで、他に
// audio/mp3・audio/ogg_opus・audio/l16・audio/alaw・audio/mulaw があります。
//
// ただし 2026-09-07 時点で、Lyria のどのモデルもこの指定を受け付けません（README の
// 「凍結の理由」を参照）。API のバリデーションは値を通し、その先のモデルが弾きます。
// 対応した日には、このライブラリは何も変えずに動きます。
const wavMIMEType = "audio/wav"

// requestBody は interactions のリクエスト本文です。
//
// モデル名は URL ではなく本文に載ります（generateContent とはそこが違います）。
type requestBody struct {
	Model             string            `json:"model"`
	Input             any               `json:"input"`
	ResponseFormat    responseFormat    `json:"response_format"`
	GenerationConfig  *generationConfig `json:"generation_config,omitempty"`
	SafetySettings    []safetySetting   `json:"safety_settings,omitempty"`
	SystemInstruction string            `json:"system_instruction,omitempty"`
}

// responseFormat は出力形式の指定です。
type responseFormat struct {
	Type     string `json:"type"`
	MIMEType string `json:"mime_type,omitempty"`
}

// generationConfig は interactions が受け付ける生成パラメータです。
//
// generateContent の GenerationConfig とは別物で、温度や TopP はありません。
// GenerateOptions のうちここに写せるものだけを渡します。
type generationConfig struct {
	Seed            *int32   `json:"seed,omitempty"`
	MaxOutputTokens int32    `json:"max_output_tokens,omitempty"`
	StopSequences   []string `json:"stop_sequences,omitempty"`
}

type safetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// contentBlock は入力・出力に共通する内容ブロックです。
//
// type で種類を分ける形なので、テキストと画像で別の構造体にはしません。
// 出力側では mime_type と data が埋まって返ります。
type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// buildRequestBody は、genai-kit の Generate と同じ入力からリクエスト本文を組み立てます。
func buildRequestBody(model, prompt string, attachments []gemini.Attachment, opts gemini.GenerateOptions) (requestBody, error) {
	input, err := buildInput(prompt, attachments)
	if err != nil {
		return requestBody{}, err
	}

	config, err := buildGenerationConfig(opts)
	if err != nil {
		return requestBody{}, err
	}

	body := requestBody{
		Model:             model,
		Input:             input,
		ResponseFormat:    responseFormat{Type: "audio", MIMEType: wavMIMEType},
		GenerationConfig:  config,
		SystemInstruction: opts.SystemPrompt,
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

// buildInput は input フィールドの値を作ります。
//
// 添付が無ければプロンプトの文字列をそのまま渡します。API は文字列とブロックの配列の
// 両方を受け付けるので、単純な生成では JSON も読みやすい形になります。
func buildInput(prompt string, attachments []gemini.Attachment) (any, error) {
	blocks := make([]contentBlock, 0, len(attachments)+1)
	if prompt != "" {
		blocks = append(blocks, contentBlock{Type: "text", Text: prompt})
	}

	for i, attachment := range attachments {
		// 送るものが無い添付は落とします。画像を「あれば渡す」形で組み立てる呼び出し側が、
		// 空要素の除去を毎回書かずに済むようにするためです（genai-kit と同じ規則）。
		if attachment.IsEmpty() {
			continue
		}
		if len(attachment.Data) > 0 && attachment.URI != "" {
			return nil, fmt.Errorf("%w: attachments[%d] は Data と URI のどちらか一方だけを設定してください", ErrInvalidAttachment, i)
		}
		if !strings.HasPrefix(attachment.MIMEType, "image/") {
			return nil, fmt.Errorf("%w: attachments[%d] の MIME type は %q", ErrUnsupportedAttachment, i, attachment.MIMEType)
		}
		if len(attachment.Data) > 0 && attachment.MIMEType == "" {
			return nil, fmt.Errorf("%w: attachments[%d] にMIME typeが設定されていません", ErrInvalidAttachment, i)
		}
		blocks = append(blocks, contentBlock{
			Type:     "image",
			MIMEType: attachment.MIMEType,
			Data:     attachment.Data,
			URI:      attachment.URI,
		})
	}

	switch len(blocks) {
	case 0:
		return nil, ErrEmptyInput
	case 1:
		if blocks[0].Type == "text" {
			return blocks[0].Text, nil
		}
	}
	return blocks, nil
}

// buildGenerationConfig は GenerateOptions のうち interactions が受け付ける項目を写します。
//
// 温度・TopP・TopK・ResponseMIMEType は interactions の generation_config に無いので
// 落ちます。音声生成で意味を持つのは Seed で、genai-kit の lyria が再現性のために渡してきます。
func buildGenerationConfig(opts gemini.GenerateOptions) (*generationConfig, error) {
	config := &generationConfig{
		MaxOutputTokens: opts.MaxOutputTokens,
		StopSequences:   opts.StopSequences,
	}
	if opts.Seed != nil {
		if *opts.Seed < math.MinInt32 || *opts.Seed > math.MaxInt32 {
			return nil, fmt.Errorf("%w: %d", ErrInvalidSeed, *opts.Seed)
		}
		seed := int32(*opts.Seed)
		config.Seed = &seed
	}
	if config.Seed == nil && config.MaxOutputTokens == 0 && len(config.StopSequences) == 0 {
		return nil, nil
	}
	return config, nil
}
