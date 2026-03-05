package config

import (
	"testing"
)

func TestDetectLocale(t *testing.T) {
	tests := []struct {
		name string
		envs map[string]string
		want string
	}{
		{
			name: "HERMES_LOCALE wins",
			envs: map[string]string{"HERMES_LOCALE": "de", "LANG": "fr_FR.UTF-8"},
			want: "de",
		},
		{
			name: "falls back to LANG",
			envs: map[string]string{"LANG": "ja_JP.UTF-8"},
			want: "ja",
		},
		{
			name: "falls back to LC_MESSAGES",
			envs: map[string]string{"LC_MESSAGES": "es_ES.UTF-8"},
			want: "es",
		},
		{
			name: "falls back to LANGUAGE",
			envs: map[string]string{"LANGUAGE": "ko"},
			want: "ko",
		},
		{
			name: "defaults to en when nothing set",
			envs: map[string]string{},
			want: "en",
		},
		{
			name: "C locale returns en",
			envs: map[string]string{"LANG": "C"},
			want: "en",
		},
		{
			name: "POSIX locale returns en",
			envs: map[string]string{"LANG": "POSIX"},
			want: "en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, env := range []string{"HERMES_LOCALE", "LANG", "LC_MESSAGES", "LANGUAGE"} {
				t.Setenv(env, "")
			}
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}
			if got := DetectLocale(); got != tt.want {
				t.Errorf("DetectLocale() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"en_US.UTF-8", "en"},
		{"ja_JP", "ja"},
		{"de", "de"},
		{"C", "en"},
		{"C.UTF-8", "en"},
		{"POSIX", "en"},
		{"", "en"},
		{"fr_FR", "fr"},
		{"zh_CN.UTF-8", "zh"},
		{"zh_TW", "zh"},
		{"x", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := normalizeLocale(tt.input); got != tt.want {
				t.Errorf("normalizeLocale(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectLocale_AllCleared(t *testing.T) {
	for _, env := range []string{"HERMES_LOCALE", "LANG", "LC_MESSAGES", "LANGUAGE"} {
		t.Setenv(env, "")
	}
	if got := DetectLocale(); got != "en" {
		t.Errorf("DetectLocale() = %q with all envs cleared, want en", got)
	}
}
