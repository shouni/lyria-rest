package lyriarest

import (
	"errors"
	"fmt"
)

// センチネルの文言は英語 + "lyriarest: " プレフィックスで統一しています。深いラップの
// 中に埋まってもどのパッケージ由来か判別できるようにするためで、人間向けの文脈は
// ラップする側（fmt.Errorf の %w）が日本語で補います。
var (
	// ErrConfigRequired は、ProjectID/LocationID と APIKey のいずれも設定されていない場合に返されます。
	ErrConfigRequired = errors.New("lyriarest: either ProjectID/LocationID or APIKey is required")
	// ErrExclusiveConfig は、ProjectID/LocationID と APIKey が同時に設定された場合に返されます。
	ErrExclusiveConfig = errors.New("lyriarest: ProjectID/LocationID and APIKey are mutually exclusive")
	// ErrIncompleteVertexConfig は、ProjectID と LocationID の一方のみが設定された場合に返されます。
	ErrIncompleteVertexConfig = errors.New("lyriarest: Vertex AI requires both ProjectID and LocationID")
	// ErrEmptyModelName は、モデル名が空の場合に返されます。
	ErrEmptyModelName = errors.New("lyriarest: model name is empty")
	// ErrEmptyParts は、プロンプトも添付も無く送るものが無い場合に返されます。
	ErrEmptyParts = errors.New("lyriarest: generation parts are empty")
	// ErrInvalidAttachment は、添付の指定が不正な場合に返されます。
	// Data と URI の併用、および Data に MIME type が無い場合が該当します。
	ErrInvalidAttachment = errors.New("lyriarest: invalid attachment")
	// ErrInvalidSeed は、Seed が int32 の範囲外の場合に返されます。
	ErrInvalidSeed = errors.New("lyriarest: seed must fit in int32")
	// ErrHTTP は、API が 2xx 以外を返したことを示します。詳細は HTTPError にあります。
	ErrHTTP = errors.New("lyriarest: request failed")
	// ErrResponseTooLarge は、レスポンス本文がサイズ上限を超えた場合に返されます。
	ErrResponseTooLarge = errors.New("lyriarest: response body exceeds the size limit")
)

// HTTPError は、API が 2xx 以外のステータスを返した場合のエラーです。
// errors.Is(err, ErrHTTP) で分類でき、StatusCode で再試行の可否を判断できます。
type HTTPError struct {
	StatusCode int
	// Body はレスポンス本文の先頭です。API のエラーメッセージがここに入ります。
	Body string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("lyriarest: HTTP %d: %s", e.StatusCode, e.Body)
}

// Unwrap は分類用センチネル ErrHTTP を返します。
func (e *HTTPError) Unwrap() error { return ErrHTTP }

// ResponseError は、API との通信は成功したがレスポンスが利用できない場合のエラーです。
//
// Reason には genai-kit の gemini.ErrBlocked / gemini.ErrEmptyResponse を入れます。
// genai-kit の経路に戻したときも呼び出し側の errors.Is が同じ分岐を通るように、
// センチネルを独自に持たず借りています。
type ResponseError struct {
	// Reason は分類用のセンチネルです。
	Reason error
	// FinishReason は、ブロック時にモデルが返した終了理由です。無ければ空文字列です。
	FinishReason string
	// Message は人間向けの説明です。
	Message string
}

func (e *ResponseError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Reason != nil {
		return e.Reason.Error()
	}
	return "lyriarest: API response error"
}

// Unwrap は分類用センチネルを返し、errors.Is による判定を可能にします。
func (e *ResponseError) Unwrap() error { return e.Reason }
