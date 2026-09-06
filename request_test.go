package lyriarest

import (
	"errors"
	"math"
	"testing"

	"github.com/shouni/genai-kit/gemini"
)

// TestBuildRequestBodyMapsPromptAndAttachments は、プロンプトと添付がパートへ写ることを
// 検証します。genai-kit の attachmentParts と同じ規則で、空の添付は読み飛ばします。
func TestBuildRequestBodyMapsPromptAndAttachments(t *testing.T) {
	t.Parallel()

	body, err := buildRequestBody("sing this", []gemini.Attachment{
		{},
		{MIMEType: "image/png", Data: []byte("cover")},
		{URI: "gs://bucket/ref.png"},
	}, gemini.GenerateOptions{})
	if err != nil {
		t.Fatalf("buildRequestBody() error = %v", err)
	}

	if len(body.Contents) != 1 || body.Contents[0].Role != "user" {
		t.Fatalf("Contents = %+v, want user ロールの 1 件", body.Contents)
	}
	parts := body.Contents[0].Parts
	if len(parts) != 3 {
		t.Fatalf("Parts = %d, want 3（空の添付は落ちる）", len(parts))
	}
	if parts[0].Text != "sing this" {
		t.Errorf("parts[0].Text = %q", parts[0].Text)
	}
	if parts[1].InlineData == nil || parts[1].InlineData.MIMEType != "image/png" || string(parts[1].InlineData.Data) != "cover" {
		t.Errorf("parts[1] = %+v, want インライン画像", parts[1])
	}
	if parts[2].FileData == nil || parts[2].FileData.FileURI != "gs://bucket/ref.png" {
		t.Errorf("parts[2] = %+v, want gs:// の fileData", parts[2])
	}
}

// TestBuildRequestBodyAlwaysRequestsWAV は、WAV 指定が必ず generationConfig に載ること、
// 呼び出し側の指定より後に上書きされることを検証します。フィールド名には依存しません。
func TestBuildRequestBodyAlwaysRequestsWAV(t *testing.T) {
	t.Parallel()

	if len(wavGenerationConfig) == 0 {
		t.Fatal("wavGenerationConfig が空です。WAV 前提のライブラリなので、指定は必ず要ります")
	}

	opts := gemini.GenerateOptions{}
	for key := range wavGenerationConfig {
		if key == "responseMimeType" {
			opts.ResponseMIMEType = "application/json"
		}
	}
	body, err := buildRequestBody("p", nil, opts)
	if err != nil {
		t.Fatalf("buildRequestBody() error = %v", err)
	}

	for key, want := range wavGenerationConfig {
		if got := body.GenerationConfig[key]; got != want {
			t.Errorf("generationConfig[%q] = %v, want %v（呼び出し側の指定より WAV が勝つこと）", key, got, want)
		}
	}
}

// TestBuildRequestBodyKeepsGenerationOptions は、Seed と安全設定が落ちないことを検証します。
// genai-kit の lyria は音声生成に Seed と BLOCK_NONE の安全設定を渡してきます。
func TestBuildRequestBodyKeepsGenerationOptions(t *testing.T) {
	t.Parallel()

	seed := int64(42)
	body, err := buildRequestBody("p", nil, gemini.GenerateOptions{
		Seed:           &seed,
		SystemPrompt:   "be quiet",
		SafetySettings: gemini.NewSafetySettings(gemini.SafetyBlockNone),
	})
	if err != nil {
		t.Fatalf("buildRequestBody() error = %v", err)
	}

	if got := body.GenerationConfig["seed"]; got != int32(42) {
		t.Errorf("seed = %v (%T), want int32(42)", got, got)
	}
	if body.SystemInstruction == nil || body.SystemInstruction.Parts[0].Text != "be quiet" {
		t.Errorf("SystemInstruction = %+v", body.SystemInstruction)
	}
	if len(body.SafetySettings) == 0 {
		t.Fatal("SafetySettings が落ちています")
	}
	for _, s := range body.SafetySettings {
		if s.Category == "" || s.Threshold != string(gemini.SafetyBlockNone) {
			t.Errorf("SafetySettings = %+v, want BLOCK_NONE", s)
		}
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
		{"送るものが無い", "", []gemini.Attachment{{}}, gemini.GenerateOptions{}, ErrEmptyParts},
		{"Data と URI の併用", "p", []gemini.Attachment{{Data: []byte("x"), URI: "gs://b/a", MIMEType: "image/png"}}, gemini.GenerateOptions{}, ErrInvalidAttachment},
		{"Data に MIME type が無い", "p", []gemini.Attachment{{Data: []byte("x")}}, gemini.GenerateOptions{}, ErrInvalidAttachment},
		{"Seed が int32 を超える", "p", nil, gemini.GenerateOptions{Seed: new(int64(math.MaxInt32 + 1))}, ErrInvalidSeed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildRequestBody(tt.prompt, tt.attachments, tt.opts)
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
