package k3client

import "testing"

// TestSha256SortedSign pins the signature to a value computed independently
// (Python: hashlib.sha256 over the lexicographically-sorted parts).
func TestSha256SortedSign(t *testing.T) {
	got := sha256SortedSign([]string{"acct1", "user1", "app1", "secret1", "1700000000"})
	want := "dd2e6d9bb803095816d0366e1231441ebc6639c7c4e9c866ae2d924953db7cef"
	if got != want {
		t.Fatalf("sha256SortedSign = %s, want %s", got, want)
	}
}

// TestBuildAttachmentDownLoadParams pins the wrapping form (one JSON-string
// element, matching ExecuteBillQuery). Verified against the live instance.
func TestBuildAttachmentDownLoadParams(t *testing.T) {
	got := BuildAttachmentDownLoadParams("99f4cb", 0)
	want := `["{\"FileId\":\"99f4cb\",\"StartIndex\":0}"]`
	if got != want {
		t.Errorf("BuildAttachmentDownLoadParams = %s, want %s", got, want)
	}
}

// TestChinaToUnicode verifies CJK runes become lower-case \uXXXX escapes while
// everything else passes through. The `in` fields are real CJK glyphs; the
// `want` fields use "\\u...." so the expected value is the literal 6-char
// backslash-u-hex sequence the function emits (not a Go-decoded rune).
func TestChinaToUnicode(t *testing.T) {
	// 追=U+8FFD 溯=U+6EAF 系=U+7CFB 统=U+7EDF
	cases := []struct{ in, want string }{
		{"追溯系统", "\\u8ffd\\u6eaf\\u7cfb\\u7edf"},
		{"ENG_BOM", "ENG_BOM"},
		{"1.30.67.0132", "1.30.67.0132"},
		{"a追b", "a\\u8ffdb"},
		{"", ""},
	}
	for _, c := range cases {
		if got := chinaToUnicode(c.in); got != c.want {
			t.Errorf("chinaToUnicode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
