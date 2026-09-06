package lyriarest

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/shouni/genai-kit/gemini"
)

// TestBuildRequestBodyAlwaysRequestsWAV は、WAV の指定が必ず載ることを検証します。
// これを送らないと SDK 経由と同じ MP3 になり、このライブラリの存在理由が消えます。
func TestBuildRequestBodyAlwaysRequestsWAV(t *testing.T) {
	t.Parallel()

	body, err := buildRequestBody("lyria-3.5", "p", nil, gemini.GenerateOptions{})
	if err != nil {
		t.Fatalf("buildRequestBody() error = %v", err)
	}

	if body.ResponseFormat.Type != "audio" || body.ResponseFormat.MIMEType != wavMIMEType {
		t.Errorf("ResponseFormat = %+v, want type=audio mime_type=%s", body.ResponseFormat, wavMIMEType)
	}
	if body.Model != "lyria-3.5" {
		t.Errorf("Model = %q（モデルは URL ではなく本文に載る）", body.Model)
	}
}

// TestBuildInputUsesPlainStringWithoutAttachments は、添付が無ければ input が
// 文字列になることを検証します。API は文字列とブロック配列の両方を受け付けます。
func TestBuildInputUsesPlainStringWithoutAttachments(t *testing.T) {
	t.Parallel()

	got, err := buildInput("sing this", nil)
	if err != nil {
		t.Fatalf("buildInput() error = %v", err)
	}
	if s, ok := got.(string); !ok || s != "sing this" {
		t.Errorf("input = %#v, want 文字列", got)
	}
}

// TestBuildInputBuildsBlocksWithAttachments は、添付があるとブロック配列になり、
// 空の添付が黙って落ちることを検証します（genai-kit と同じ規則）。
func TestBuildInputBuildsBlocksWithAttachments(t *testing.T) {
	t.Parallel()

	got, err := buildInput("sing this", []gemini.Attachment{
		{},
		{MIMEType: "image/png", Data: []byte("cover")},
	})
	if err != nil {
		t.Fatalf("buildInput() error = %v", err)
	}

	blocks, ok := got.([]contentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("input = %#v, want 2 ブロック（空の添付は落ちる）", got)
	}
	if blocks[0].Type != "text" || blocks[0].Text != "sing this" {
		t.Errorf("blocks[0] = %+v", blocks[0])
	}
	if blocks[1].Type != "image" || blocks[1].MIMEType != "image/png" || string(blocks[1].Data) != "cover" {
		t.Errorf("blocks[1] = %+v", blocks[1])
	}

	// data は base64 の文字列として送られること（[]byte の既定の符号化）。
	raw, err := json.Marshal(blocks[1])
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded["data"] != "Y292ZXI=" {
		t.Errorf("data = %v, want base64 文字列", decoded["data"])
	}
}

// TestBuildGenerationConfigKeepsSeed は、シードが落ちないことを検証します。
// genai-kit の lyria は再現性のためにシードを渡してきます。
func TestBuildGenerationConfigKeepsSeed(t *testing.T) {
	t.Parallel()

	seed := int64(42)
	got, err := buildGenerationConfig(gemini.GenerateOptions{Seed: &seed})
	if err != nil {
		t.Fatalf("buildGenerationConfig() error = %v", err)
	}
	if got == nil || got.Seed == nil || *got.Seed != 42 {
		t.Errorf("generation_config = %+v, want seed=42", got)
	}
}

// TestBuildGenerationConfigOmittedWhenEmpty は、渡すものが無ければ
// generation_config そのものを送らないことを検証します。
func TestBuildGenerationConfigOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	got, err := buildGenerationConfig(gemini.GenerateOptions{Temperature: new(float32(0.5))})
	if err != nil {
		t.Fatalf("buildGenerationConfig() error = %v", err)
	}
	if got != nil {
		t.Errorf("generation_config = %+v, want nil（interactions に温度は無い）", got)
	}
}

func TestBuildRequestBodyRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prompt      string
		attachments []gemini.Attachment
		opts        gemini.GenerateOptions
		want        error
	}{
		{"送るものが無い", "", []gemini.Attachment{{}}, gemini.GenerateOptions{}, ErrEmptyInput},
		{"Data と URI の併用", "p", []gemini.Attachment{{Data: []byte("x"), URI: "https://e/a.png", MIMEType: "image/png"}}, gemini.GenerateOptions{}, ErrInvalidAttachment},
		{"画像以外の添付", "p", []gemini.Attachment{{Data: []byte("x"), MIMEType: "audio/mpeg"}}, gemini.GenerateOptions{}, ErrUnsupportedAttachment},
		{"Seed が int32 を超える", "p", nil, gemini.GenerateOptions{Seed: new(int64(math.MaxInt32 + 1))}, ErrInvalidSeed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildRequestBody("lyria-3.5", tt.prompt, tt.attachments, tt.opts)
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
