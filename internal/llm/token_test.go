package llm

import "testing"

func TestEstimateTokenCount_ASCII(t *testing.T) {
	text := "Hello world this is a test sentence"
	tokens := EstimateTokenCount(text)
	if tokens < 5 {
		t.Errorf("tokens = %d, expected at least 5 for ASCII text", tokens)
	}
}

func TestEstimateTokenCount_CJK(t *testing.T) {
	text := "これはテストです"
	tokens := EstimateTokenCount(text)
	// 8 CJK chars × 2 = 16
	if tokens < 10 {
		t.Errorf("tokens = %d, expected at least 10 for CJK text", tokens)
	}
}

func TestEstimateTokenCount_JSON(t *testing.T) {
	// JSON has heavy punctuation — char-based should dominate
	text := `{"key1":"val1","key2":"val2","key3":"val3","key4":"val4"}`
	tokens := EstimateTokenCount(text)
	charBased := len(text) / 4 // 56/4 = 14
	if tokens < charBased {
		t.Errorf("tokens = %d, should be at least char-based %d for JSON", tokens, charBased)
	}
}

func TestEstimateTokenCount_Empty(t *testing.T) {
	tokens := EstimateTokenCount("")
	if tokens != 0 {
		t.Errorf("tokens = %d, want 0 for empty string", tokens)
	}
}

func TestEstimateTokenCount_Mixed(t *testing.T) {
	text := `ユーザーが "hello" と入力した場合`
	tokens := EstimateTokenCount(text)
	if tokens < 10 {
		t.Errorf("tokens = %d, expected at least 10 for mixed text", tokens)
	}
}
