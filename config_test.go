package lyriarest

import (
	"errors"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	if err := (Config{}).validate(); !errors.Is(err, ErrAPIKeyRequired) {
		t.Errorf("validate() error = %v, want %v", err, ErrAPIKeyRequired)
	}
	if err := (Config{APIKey: "key"}).validate(); err != nil {
		t.Errorf("validate() error = %v, want nil", err)
	}
}

// TestInteractionsURL は、モデル名が URL に入らないことを検証します。
// generateContent とは違い、interactions はモデルをリクエスト本文で指定します。
func TestInteractionsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{"既定", Config{APIKey: "k"}, "https://generativelanguage.googleapis.com/v1beta/interactions"},
		{"Endpoint 上書き（末尾スラッシュは落とす）", Config{APIKey: "k", Endpoint: "http://127.0.0.1:8080/"}, "http://127.0.0.1:8080/v1beta/interactions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.cfg.interactionsURL(); got != tt.want {
				t.Errorf("interactionsURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNormalizeModel は、SDK が受け付ける 2 通りの表記が同じ値になることを検証します。
func TestNormalizeModel(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"lyria-3.5", "models/lyria-3.5", "  lyria-3.5 "} {
		if got := normalizeModel(in); got != "lyria-3.5" {
			t.Errorf("normalizeModel(%q) = %q", in, got)
		}
	}
}
