package lyriarest

import (
	"errors"
	"testing"

	"github.com/shouni/genai-kit/gemini"
)

// TestParseResponseJoinsTextAcrossSteps は、複数の step に分かれたテキストが連結され、
// model_output 以外の step が読み飛ばされることを検証します。
//
// Lyria は譜面テキストと音声を別の step で返すため、step をまたいで集める必要があります。
func TestParseResponseJoinsTextAcrossSteps(t *testing.T) {
	t.Parallel()

	got, err := parseResponse([]byte(`{"status":"completed","steps":[
	  {"type":"user_input","content":[{"type":"text","text":"入力は拾わない"}]},
	  {"type":"model_output","content":[{"type":"text","text":"[[A0]]"}]},
	  {"type":"model_output","content":[{"type":"text","text":"\n[[B1]]"}]}
	]}`))
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if got.Text != "[[A0]]\n[[B1]]" {
		t.Errorf("Text = %q, want model_output だけを連結したもの", got.Text)
	}
}

// TestParseResponseSortsInlineDataByMIMEType は、インラインデータが Attachments に入り、
// 音声と画像が振り分けられることを検証します。
func TestParseResponseSortsInlineDataByMIMEType(t *testing.T) {
	t.Parallel()

	// "Y292ZXI=" = "cover", "c25k" = "snd"
	got, err := parseResponse([]byte(`{"status":"completed","steps":[{"type":"model_output","content":[
	  {"type":"image","mime_type":"image/png","data":"Y292ZXI="},
	  {"type":"audio","mime_type":"audio/wav","data":"c25k"}
	]}]}`))
	if err != nil {
		t.Fatalf("parseResponse() error = %v", err)
	}

	if len(got.Attachments) != 2 {
		t.Fatalf("Attachments = %+v, want 2 件", got.Attachments)
	}
	if len(got.Images) != 1 || string(got.Images[0]) != "cover" {
		t.Errorf("Images = %q", got.Images)
	}
	if len(got.Audios) != 1 || string(got.Audios[0]) != "snd" {
		t.Errorf("Audios = %q", got.Audios)
	}
}

// TestParseResponseRejectsEmpty は、何も返らなかった場合を genai-kit のセンチネルで
// 分類できることを検証します。
func TestParseResponseRejectsEmpty(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{"status":"completed","steps":[]}`, `{}`} {
		_, err := parseResponse([]byte(body))
		if !errors.Is(err, gemini.ErrEmptyResponse) {
			t.Errorf("parseResponse(%s) error = %v, want %v", body, err, gemini.ErrEmptyResponse)
		}
	}
}

// TestParseAPIError は、エラー本文からメッセージを取り出せることと、
// 解釈できない本文では先頭を返すことを検証します。
func TestParseAPIError(t *testing.T) {
	t.Parallel()

	got := parseAPIError([]byte(`{"error":{"message":"quota exceeded","code":"resource_exhausted"}}`), 100)
	if got != "quota exceeded" {
		t.Errorf("parseAPIError() = %q", got)
	}

	if got := parseAPIError([]byte("<html>502</html>"), 8); got != "<html>50" {
		t.Errorf("parseAPIError() = %q, want 先頭 8 バイト", got)
	}
}
