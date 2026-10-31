package collector

import "unicode/utf8"

// MaxNameBytes bounds names DockGraph keeps from sources it does not
// control, such as container names in Docker events. Docker does not bound
// them, and every copy kept in a history multiplies their size; real names
// are far shorter (DNS names stop at 253 bytes).
const MaxNameBytes = 255

// CapText returns s cut to at most limit bytes, never inside a UTF-8
// sequence. A limit of zero or less returns "".
func CapText(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
