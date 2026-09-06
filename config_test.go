package lyriarest

import (
	"errors"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want error
	}{
		{"正常系: Vertex AI", Config{ProjectID: "p", LocationID: "asia-northeast1"}, nil},
		{"正常系: Gemini API", Config{APIKey: "key"}, nil},
		{"異常系: どちらも空", Config{}, ErrConfigRequired},
		{"異常系: ProjectID のみ", Config{ProjectID: "p"}, ErrIncompleteVertexConfig},
		{"異常系: LocationID のみ", Config{LocationID: "l"}, ErrIncompleteVertexConfig},
		// どちらを使うか決められないため、黙って一方を選ばずに落とす。
		{"異常系: 併用", Config{ProjectID: "p", LocationID: "l", APIKey: "key"}, ErrExclusiveConfig},
		// 書きかけの Vertex 設定より、併用そのものを先に知らせる。
		{"異常系: 併用 + 書きかけ", Config{ProjectID: "p", APIKey: "key"}, ErrExclusiveConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.validate()
			if tt.want == nil {
				if err != nil {
					t.Fatalf("validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestGenerateContentURL は、バックエンドごとの URL の形を検証します。
// Vertex AI の "global" だけはホストにリージョン接頭辞が付きません。
func TestGenerateContentURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		cfg   Config
		model string
		want  string
	}{
		{
			name:  "Gemini API",
			cfg:   Config{APIKey: "key"},
			model: "lyria-3.5",
			want:  "https://generativelanguage.googleapis.com/v1beta/models/lyria-3.5:generateContent",
		},
		{
			name:  "Vertex AI（リージョン）",
			cfg:   Config{ProjectID: "proj", LocationID: "us-central1"},
			model: "lyria-3.5",
			want:  "https://us-central1-aiplatform.googleapis.com/v1/projects/proj/locations/us-central1/publishers/google/models/lyria-3.5:generateContent",
		},
		{
			name:  "Vertex AI（global はホストに接頭辞なし）",
			cfg:   Config{ProjectID: "proj", LocationID: "global"},
			model: "lyria-3.5",
			want:  "https://aiplatform.googleapis.com/v1/projects/proj/locations/global/publishers/google/models/lyria-3.5:generateContent",
		},
		{
			name:  "Endpoint 上書き（末尾スラッシュは落とす）",
			cfg:   Config{APIKey: "key", Endpoint: "http://127.0.0.1:8080/"},
			model: "lyria-3.5",
			want:  "http://127.0.0.1:8080/v1beta/models/lyria-3.5:generateContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.cfg.generateContentURL(tt.model); got != tt.want {
				t.Errorf("generateContentURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeModel は、SDK が受け付ける 2 通りの表記が同じ URL になることを検証します。
func TestNormalizeModel(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"lyria-3.5", "models/lyria-3.5", "  lyria-3.5 "} {
		if got := normalizeModel(in); got != "lyria-3.5" {
			t.Errorf("normalizeModel(%q) = %q", in, got)
		}
	}
	if got := normalizeModel(""); got != "" {
		t.Errorf("normalizeModel(\"\") = %q, want empty", got)
	}
}
