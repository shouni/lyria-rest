package lyriarest

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/shouni/genai-kit/gemini"
)

// TestParseResponseExtractsAudioTextAndUsage は、音声・テキスト・トークン使用量が
// genai-kit の Response と同じ場所に入ることを検証します。genai-kit の lyria は
// Attachments の MIME type が audio/ で始まる要素を音声として拾い、Text を譜面として使います。
func TestParseResponseExtractsAudioTextAndUsage(t *testing.T) {
	t.Parallel()

	wav := []byte("RIFF....WAVEfmt ")
	data := []byte(`{
	  "candidates": [{
	    "content": {"role": "model", "parts": [
	      {"text": "[[V1]]\n[:] sung line"},
	      {"inlineData": {"mimeType": "audio/wav", "data": "` + base64.StdEncoding.EncodeToString(wav) + `"}}
	    ]},
	    "finishReason": "STOP"
	  }],
	  "usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5, "totalTokenCount": 15}
	}`)

	got, err := parseResponse(data)
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if got.Text != "[[V1]]\n[:] sung line" {
		t.Errorf("Text = %q", got.Text)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].MIMEType != "audio/wav" || string(got.Attachments[0].Data) != string(wav) {
		t.Errorf("Attachments = %+v, want audio/wav 1 件（base64 を戻したバイト列）", got.Attachments)
	}
	if len(got.Audios) != 1 || len(got.Images) != 0 {
		t.Errorf("Audios = %d, Images = %d, want 1 / 0", len(got.Audios), len(got.Images))
	}
	if got.Usage == nil || got.Usage.TotalTokenCount != 15 {
		t.Errorf("Usage = %+v", got.Usage)
	}
}

// TestParseResponseSeparatesThoughts は、思考パートが Text に混ざらないことを検証します。
// 混ざると genai-kit の lyria が譜面として突き合わせる文字列に思考が入ります。
func TestParseResponseSeparatesThoughts(t *testing.T) {
	t.Parallel()

	got, err := parseResponse([]byte(`{"candidates":[{"content":{"parts":[
	  {"text":"thinking...","thought":true},
	  {"text":"answer"}
	]}}]}`))
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if got.Text != "answer" || got.Thoughts != "thinking..." {
		t.Errorf("Text = %q, Thoughts = %q", got.Text, got.Thoughts)
	}
}

// TestParseResponseClassifiesFailures は、失敗の分類が genai-kit のセンチネルで判定できる
// ことを検証します。genai-kit の経路に戻しても、呼び出し側の errors.Is は同じ分岐を通ります。
func TestParseResponseClassifiesFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		want         error
		finishReason string
	}{
		{"候補が無い", `{}`, gemini.ErrEmptyResponse, ""},
		{"プロンプト側でブロック", `{"promptFeedback":{"blockReason":"SAFETY"}}`, gemini.ErrBlocked, "SAFETY"},
		{"終了理由がブロック", `{"candidates":[{"finishReason":"SAFETY"}]}`, gemini.ErrBlocked, "SAFETY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseResponse([]byte(tt.body))
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			var respErr *ResponseError
			if !errors.As(err, &respErr) {
				t.Fatalf("error = %T, want *ResponseError", err)
			}
			if respErr.FinishReason != tt.finishReason {
				t.Errorf("FinishReason = %q, want %q", respErr.FinishReason, tt.finishReason)
			}
		})
	}
}

// TestParseResponseTreatsUnsetFinishReasonAsNormal は、終了理由の「未設定」が 2 通りある
// ことへの対処を検証します。サーバーは終了理由を含めないことがあり、定数の
// FINISH_REASON_UNSPECIFIED とは別の値（空文字列）になります。
func TestParseResponseTreatsUnsetFinishReasonAsNormal(t *testing.T) {
	t.Parallel()

	for _, reason := range []string{"", "FINISH_REASON_UNSPECIFIED", "STOP"} {
		if isBlockedFinishReason(reason) {
			t.Errorf("isBlockedFinishReason(%q) = true, want false", reason)
		}
	}
	if !isBlockedFinishReason("MAX_TOKENS") {
		t.Error("isBlockedFinishReason(MAX_TOKENS) = false, want true")
	}
}
