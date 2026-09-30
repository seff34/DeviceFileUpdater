package shell

import "testing"

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"/opt/app/x.bin": `'/opt/app/x.bin'`,
		"a b":            `'a b'`,
		"it's":           `'it'\''s'`,
		"":               `''`,
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}
