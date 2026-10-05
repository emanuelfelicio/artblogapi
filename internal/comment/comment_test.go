package comment

import (
	"errors"
	"testing"
)

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "trims content", input: "  hello  ", want: "hello"},
		{name: "accepts unicode characters", input: "ação", want: "ação"},
		{name: "rejects empty content", input: "   ", wantErr: true},
		{name: "rejects content over limit", input: string(make([]rune, MaxContentLength+1)), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateContent(test.input)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidContent) {
					t.Fatalf("ValidateContent() error = %v, want ErrInvalidContent", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateContent() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ValidateContent() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizePagination(t *testing.T) {
	if limit, offset := NormalizePagination(0, -1); limit != DefaultPageLimit || offset != 0 {
		t.Fatalf("NormalizePagination() = %d, %d", limit, offset)
	}
	if limit, offset := NormalizePagination(MaxPageLimit+1, 4); limit != DefaultPageLimit || offset != 4 {
		t.Fatalf("NormalizePagination() = %d, %d", limit, offset)
	}
	if limit, offset := NormalizePagination(10, 3); limit != 10 || offset != 3 {
		t.Fatalf("NormalizePagination() = %d, %d", limit, offset)
	}
}
