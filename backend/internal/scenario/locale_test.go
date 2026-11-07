package scenario

import "testing"

func TestLocalePattern(t *testing.T) {
	for _, ok := range []string{"en", "es-MX", "pt-BR", "zh-Hant-TW", "es-419", "fil", "EN-us"} {
		if !localePattern.MatchString(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{"", "e", "en-USA-MX", "spanish", "en_US", "en-", "es-MX-extra"} {
		if localePattern.MatchString(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
