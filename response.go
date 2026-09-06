package lyriarest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shouni/genai-kit/gemini"
)

// statusCompleted は、生成が完了した interaction の状態です。
const statusCompleted = "completed"

// responseBody は interactions のレスポンス本文のうち、このライブラリが読む部分です。
type responseBody struct {
	Status string    `json:"status"`
	Steps  []step    `json:"steps"`
	Usage  *usage    `json:"usage"`
	Error  *apiError `json:"error"`
	Model  string    `json:"model"`
}

// step は 1 段の入出力です。生成結果は type が "model_output" のものに載ります。
//
// Lyria は譜面テキストと音声を別の step に分けて返します（テキストが先）。
type step struct {
	Type    string         `json:"type"`
	Content []contentBlock `json:"content"`
}

type usage struct {
	TotalTokens        int32 `json:"total_tokens"`
	TotalInputTokens   int32 `json:"total_input_tokens"`
	TotalOutputTokens  int32 `json:"total_output_tokens"`
	TotalThoughtTokens int32 `json:"total_thought_tokens"`
}

// apiError は interactions のエラー本文です。
// code は generateContent と違って文字列です（"invalid_request" など）。
type apiError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// parseResponse は、レスポンス本文を genai-kit の gemini.Response へ変換します。
//
// テキストブロックは Text へ、音声などのインラインデータは MIME type 付きで Attachments へ
// 入れ、そのうち画像と音声は Images / Audios へも振り分けます。genai-kit の lyria は
// Attachments の中で MIME type が "audio/" で始まる最初の要素を音声として拾います。
func parseResponse(data []byte) (*gemini.Response, error) {
	var body responseBody
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("lyriarest: レスポンスの解釈に失敗しました: %w", err)
	}

	if body.Status != "" && body.Status != statusCompleted {
		return nil, &ResponseError{
			Reason:  gemini.ErrEmptyResponse,
			Status:  body.Status,
			Message: fmt.Sprintf("lyriarest: interaction が %s のまま返りました", body.Status),
		}
	}

	var text strings.Builder
	var attachments []gemini.Attachment
	var images, audios [][]byte
	for _, s := range body.Steps {
		if s.Type != "model_output" {
			continue
		}
		for _, c := range s.Content {
			if c.Type == "text" {
				text.WriteString(c.Text)
				continue
			}
			if len(c.Data) == 0 {
				continue
			}
			attachment := gemini.Attachment{MIMEType: c.MIMEType, Data: c.Data}
			attachments = append(attachments, attachment)
			switch {
			case strings.HasPrefix(attachment.MIMEType, "image/"):
				images = append(images, attachment.Data)
			case strings.HasPrefix(attachment.MIMEType, "audio/"):
				audios = append(audios, attachment.Data)
			}
		}
	}

	if len(attachments) == 0 && text.Len() == 0 {
		return nil, &ResponseError{
			Reason:  gemini.ErrEmptyResponse,
			Message: "lyriarest: 空のレスポンスが返されました",
		}
	}

	return &gemini.Response{
		Text:        text.String(),
		Images:      images,
		Audios:      audios,
		Attachments: attachments,
		Usage:       tokenUsage(body.Usage),
	}, nil
}

// parseAPIError は、2xx 以外の本文からエラーメッセージを取り出します。
// 解釈できなければ本文の先頭をそのまま返します。
func parseAPIError(data []byte, limit int) string {
	var body responseBody
	if err := json.Unmarshal(data, &body); err == nil && body.Error != nil && body.Error.Message != "" {
		return body.Error.Message
	}
	if len(data) > limit {
		data = data[:limit]
	}
	return string(data)
}

func tokenUsage(u *usage) *gemini.TokenUsage {
	if u == nil {
		return nil
	}
	return &gemini.TokenUsage{
		PromptTokenCount:     u.TotalInputTokens,
		CandidatesTokenCount: u.TotalOutputTokens,
		TotalTokenCount:      u.TotalTokens,
		ThoughtsTokenCount:   u.TotalThoughtTokens,
	}
}
