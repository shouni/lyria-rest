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

// TestParseAPIError は、エラー本文からコードとメッセージを取り出せることと、
// 解釈できない本文では先頭を返すことを検証します。
//
// code は interactions では文字列、ゲートウェイ由来（認証失敗など）では数値で返ります。
// 数値の code で Unmarshal ごと失敗して本文が生のまま返るのが以前の挙動でした。
func TestParseAPIError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		limit       int
		wantCode    string
		wantMessage string
	}{
		{"interactions の文字列 code", `{"error":{"message":"quota exceeded","code":"resource_exhausted"}}`, 100, "resource_exhausted", "quota exceeded"},
		{"ゲートウェイの数値 code", `{"error":{"code":401,"message":"API key not valid.","status":"UNAUTHENTICATED"}}`, 100, "401", "API key not valid."},
		{"code 無し", `{"error":{"message":"oops"}}`, 100, "", "oops"},
		{"解釈できない本文は先頭を返す", "<html>502</html>", 8, "", "<html>50"},
		{"マルチバイト文字の途中では切らない", "エラーです", 4, "", "エ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			code, message := parseAPIError([]byte(tt.body), tt.limit)
			if code != tt.wantCode || message != tt.wantMessage {
				t.Errorf("parseAPIError() = (%q, %q), want (%q, %q)", code, message, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

// TestParseResponseIncompleteStatus は、completed 以外の状態が gemini.ErrEmptyResponse と
// ErrIncomplete の両方で判定できることを検証します。
func TestParseResponseIncompleteStatus(t *testing.T) {
	t.Parallel()

	_, err := parseResponse([]byte(`{"status":"failed","steps":[]}`))

	if !errors.Is(err, gemini.ErrEmptyResponse) {
		t.Errorf("errors.Is(err, gemini.ErrEmptyResponse) = false, err = %v", err)
	}
	if !errors.Is(err, ErrIncomplete) {
		t.Errorf("errors.Is(err, ErrIncomplete) = false, err = %v", err)
	}
	respErr, ok := errors.AsType[*ResponseError](err)
	if !ok || respErr.Status != "failed" {
		t.Errorf("error = %v, want ResponseError{Status: failed}", err)
	}

	// 空レスポンスは「完了したが中身が無い」なので ErrIncomplete にはならない。
	_, err = parseResponse([]byte(`{"status":"completed","steps":[]}`))
	if errors.Is(err, ErrIncomplete) {
		t.Errorf("空レスポンスが ErrIncomplete と判定された: %v", err)
	}
}
