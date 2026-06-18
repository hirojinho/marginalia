package agent

import "unicode/utf8"

// TruncateRunes returns s if its byte length ≤ maxBytes, else the longest
// rune-boundary-aligned prefix of s that fits within (maxBytes - 3) bytes
// followed by "…" (3 bytes in UTF-8). If maxBytes < 3 the ellipsis is
// dropped and the prefix is truncated to fit the budget directly.
//
// The function never returns invalid UTF-8 even when maxBytes lands inside
// a multi-byte rune.
func TruncateRunes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	const ellipsis = "…"
	if maxBytes < len(ellipsis) {
		return truncateNoEllipsis(s, maxBytes)
	}
	prefix := truncateNoEllipsis(s, maxBytes-len(ellipsis))
	return prefix + ellipsis
}

// truncateNoEllipsis returns the longest valid-UTF-8 prefix of s whose byte
// length is ≤ maxBytes.
func truncateNoEllipsis(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	end := 0
	for end < len(s) {
		_, size := utf8.DecodeRuneInString(s[end:])
		if end+size > maxBytes {
			break
		}
		end += size
	}
	return s[:end]
}
