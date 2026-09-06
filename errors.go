package lyriarest

import (
	"errors"
	"fmt"
)

// センチネルの文言は英語 + "lyriarest: " プレフィックスで統一しています。深いラップの
// 中に埋まってもどのパッケージ由来か判別できるようにするためで、人間向けの文脈は
// ラップする側（fmt.Errorf の %w）が日本語で補います。
var (
	// ErrAPIKeyRequired は、Config.APIKey が空の場合に返されます。
	ErrAPIKeyRequired = errors.New("lyriarest: APIKey is required")
	// ErrEmptyModelName は、モデル名が空の場合に返されます。
	ErrEmptyModelName = errors.New("lyriarest: model name is empty")
	// ErrEmptyInput は、プロンプトも添付も無く送るものが無い場合に返されます。
	ErrEmptyInput = errors.New("lyriarest: input is empty")
	// ErrInvalidAttachment は、添付の指定が不正な場合に返されます。
	// Data と URI の併用、および Data に MIME type が無い場合が該当します。
	ErrInvalidAttachment = errors.New("lyriarest: invalid attachment")
	// ErrUnsupportedAttachment は、扱えない種類の添付が渡された場合に返されます。
	//
	// interactions の入力ブロックは種類ごとに型が分かれています。このパッケージは
	// Lyria の用途で確認できた image だけを送ります。
	ErrUnsupportedAttachment = errors.New("lyriarest: only image attachments are supported")
	// ErrInvalidSeed は、Seed が int32 の範囲外の場合に返されます。
	ErrInvalidSeed = errors.New("lyriarest: seed must fit in int32")
	// ErrHTTP は、API が 2xx 以外を返したことを示します。詳細は HTTPError にあります。
	ErrHTTP = errors.New("lyriarest: request failed")
	// ErrResponseTooLarge は、レスポンス本文がサイズ上限を超えた場合に返されます。
	ErrResponseTooLarge = errors.New("lyriarest: response body exceeds the size limit")
	// ErrIncomplete は、interaction が completed 以外の状態で返された場合に返されます。
	ErrIncomplete = errors.New("lyriarest: interaction did not complete")
)

// HTTPError は、API が 2xx 以外のステータスを返した場合のエラーです。
// errors.Is(err, ErrHTTP) で分類でき、StatusCode で再試行の可否を判断できます。
//
// WAV が拒否される現在の状態もここに現れます（HTTP 400 と
// "Audio MIME type AUDIO_WAV is not supported for models/lyria-3.5"）。
type HTTPError struct {
	StatusCode int
	// Message は API が返したエラーメッセージです。取り出せなければ本文の先頭が入ります。
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("lyriarest: HTTP %d: %s", e.StatusCode, e.Message)
}

// Unwrap は分類用センチネル ErrHTTP を返します。
func (e *HTTPError) Unwrap() error { return ErrHTTP }

// ResponseError は、API との通信は成功したがレスポンスが利用できない場合のエラーです。
//
// Reason には genai-kit の gemini.ErrEmptyResponse を入れます。genai-kit の経路に戻した
// ときも呼び出し側の errors.Is が同じ分岐を通るように、センチネルを独自に持たず借りています。
type ResponseError struct {
	// Reason は分類用のセンチネルです。
	Reason error
	// Status は interaction の状態です（"completed" 以外のときに入ります）。
	Status string
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
