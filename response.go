package lyriarest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shouni/genai-kit/gemini"
)

// responseBody は generateContent のレスポンス本文のうち、このライブラリが読む部分です。
type responseBody struct {
	Candidates     []candidate     `json:"candidates"`
	UsageMetadata  *usageMetadata  `json:"usageMetadata"`
	PromptFeedback *promptFeedback `json:"promptFeedback"`
}

type candidate struct {
	Content      *content `json:"content"`
	FinishReason string   `json:"finishReason"`
}

type usageMetadata struct {
	PromptTokenCount     int32 `json:"promptTokenCount"`
	CandidatesTokenCount int32 `json:"candidatesTokenCount"`
	TotalTokenCount      int32 `json:"totalTokenCount"`
	ThoughtsTokenCount   int32 `json:"thoughtsTokenCount"`
}

type promptFeedback struct {
	BlockReason string `json:"blockReason"`
}

// parseResponse は、レスポンス本文を genai-kit の gemini.Response へ変換します。
//
// 判定の規則は genai-kit の responseFromGenAI と揃えています。候補が無ければ空レスポンス、
// 終了理由が未設定でも STOP でもなければブロック、Thought のパートは Text に含めず
// Thoughts へ、インラインデータは MIME type 付きで Attachments へ、そのうち画像と音声は
// Images / Audios へも振り分けます。
func parseResponse(data []byte) (*gemini.Response, error) {
	var body responseBody
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("lyriarest: レスポンスの解釈に失敗しました: %w", err)
	}

	if len(body.Candidates) == 0 {
		if body.PromptFeedback != nil && body.PromptFeedback.BlockReason != "" {
			return nil, newBlockedError(body.PromptFeedback.BlockReason)
		}
		return nil, &ResponseError{Reason: gemini.ErrEmptyResponse, Message: "lyriarest: 空のレスポンスが返されました"}
	}

	first := body.Candidates[0]
	if isBlockedFinishReason(first.FinishReason) {
		return nil, newBlockedError(first.FinishReason)
	}

	var text, thoughts strings.Builder
	var attachments []gemini.Attachment
	var images, audios [][]byte
	if first.Content != nil {
		for _, p := range first.Content.Parts {
			switch {
			case p.InlineData != nil:
				attachment := gemini.Attachment{MIMEType: p.InlineData.MIMEType, Data: p.InlineData.Data}
				attachments = append(attachments, attachment)
				switch {
				case strings.HasPrefix(attachment.MIMEType, "image/"):
					images = append(images, attachment.Data)
				case strings.HasPrefix(attachment.MIMEType, "audio/"):
					audios = append(audios, attachment.Data)
				}
			case p.Text == "":
				continue
			case p.Thought:
				thoughts.WriteString(p.Text)
			default:
				text.WriteString(p.Text)
			}
		}
	}

	return &gemini.Response{
		Text:        text.String(),
		Images:      images,
		Audios:      audios,
		Attachments: attachments,
		Thoughts:    thoughts.String(),
		Usage:       tokenUsage(body.UsageMetadata),
	}, nil
}

// isBlockedFinishReason は、終了理由が異常終了（ブロック等）を示すかを返します。
//
// 未設定は "" と "FINISH_REASON_UNSPECIFIED" の 2 通りあります。サーバーは終了理由を
// 含めないことがあり、そのとき空文字列になるため、両方を正常として扱います。
func isBlockedFinishReason(reason string) bool {
	switch reason {
	case "", "FINISH_REASON_UNSPECIFIED", "STOP":
		return false
	}
	return true
}

func newBlockedError(reason string) *ResponseError {
	return &ResponseError{
		Reason:       gemini.ErrBlocked,
		FinishReason: reason,
		Message:      fmt.Sprintf("lyriarest: 生成がブロックされました（理由: %s）", reason),
	}
}

func tokenUsage(meta *usageMetadata) *gemini.TokenUsage {
	if meta == nil {
		return nil
	}
	return &gemini.TokenUsage{
		PromptTokenCount:     meta.PromptTokenCount,
		CandidatesTokenCount: meta.CandidatesTokenCount,
		TotalTokenCount:      meta.TotalTokenCount,
		ThoughtsTokenCount:   meta.ThoughtsTokenCount,
	}
}
