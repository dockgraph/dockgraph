package secrets

import (
	"regexp"
	"strings"
)

var (
	// jsonMember matches a JSON member with a string value ("key": "value").
	jsonMember = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"(\s*:\s*)"(?:[^"\\]|\\.)*"`)
	// escapedJSONMember matches the same inside a double-quoted shell string,
	// where every quote is escaped (\"key\":\"value\").
	escapedJSONMember = regexp.MustCompile(`\\"([^"\\]*)\\"(\s*:\s*)\\"[^"\\]*\\"`)
)

// jsonNumberMember matches a JSON member with a number value ("pin": 1234).
var jsonNumberMember = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"(\s*:\s*)-?[0-9][0-9.eE+\-]*`)

// maskJSONMembers masks the values of JSON members with sensitive keys: the
// string values, in plain JSON and in JSON written inside a shell string,
// and the number values, replaced by the mask as a string.
func maskJSONMembers(s string) string {
	if !strings.Contains(s, `"`) || !strings.Contains(s, ":") {
		return s
	}
	s = maskMembers(s, escapedJSONMember, `\"`)
	s = maskMembers(s, jsonMember, `"`)
	return jsonNumberMember.ReplaceAllStringFunc(s, func(match string) string {
		parts := jsonNumberMember.FindStringSubmatch(match)
		if !IsSensitiveKey(parts[1]) {
			return match
		}
		return `"` + parts[1] + `"` + parts[2] + `"` + Masked + `"`
	})
}

// xmlElement matches an XML element holding text only
// ("<password>…</password>"), capturing its name, attributes, text and
// closing name.
var xmlElement = regexp.MustCompile(`<([A-Za-z_][A-Za-z0-9_.\-:]*)(\s[^<>]*)?>([^<]*)</([A-Za-z_][A-Za-z0-9_.\-:]*)\s*>`)

// maskXMLElements masks the text of XML elements with sensitive names.
func maskXMLElements(s string) string {
	if !strings.Contains(s, "</") {
		return s
	}
	return xmlElement.ReplaceAllStringFunc(s, func(match string) string {
		loc := xmlElement.FindStringSubmatchIndex(match)
		name, closing := match[loc[2]:loc[3]], match[loc[8]:loc[9]]
		if name != closing || !IsSensitiveKey(name) {
			return match
		}
		return match[:loc[6]] + Masked + match[loc[7]:]
	})
}

var (
	// goDSN matches the user and password of a Go MySQL driver DSN
	// ("user:password@tcp(db:3306)/app"), up to the last "@" before the
	// protocol.
	goDSN = regexp.MustCompile(`([^:@\s'"=,;]*):(\S*)@((?:tcp|tcp4|tcp6|udp|unix)\()`)
	// goDSNProtocol matches what follows the password of such a DSN.
	goDSNProtocol = regexp.MustCompile(`@(?:tcp|tcp4|tcp6|udp|unix)\(`)
	// oracleURL matches the user and password of an Oracle JDBC URL
	// ("jdbc:oracle:thin:user/password@db:1521:orcl").
	oracleURL = regexp.MustCompile(`(jdbc:oracle:[A-Za-z0-9]+:)([^/@\s:]*)/([^@\s]*)@`)
)

// maskDSNs masks the passwords of Go MySQL DSNs and Oracle JDBC URLs in w.
func maskDSNs(w string) string {
	if strings.Contains(w, "jdbc:oracle:") {
		w = oracleURL.ReplaceAllString(w, "${1}${2}/"+Masked+"@")
	}
	if goDSNProtocol.MatchString(w) {
		w = goDSN.ReplaceAllString(w, "${1}:"+Masked+"@${3}")
	}
	return w
}

// maskMembers masks the value of every match of member, a pattern capturing
// a member's key and the separator after it, whose key is sensitive. quote is
// how the pattern's strings are quoted.
func maskMembers(s string, member *regexp.Regexp, quote string) string {
	var b strings.Builder
	for {
		loc := member.FindStringSubmatchIndex(s)
		if loc == nil {
			b.WriteString(s)
			return b.String()
		}
		// In text that is not quite JSON, what looks like a member may
		// overlap a real one: its value may be the key of a real one, or end
		// on the quote that opens one. So scanning goes on from the value of
		// a member left as it is, and from the closing quote of a masked one.
		// It never goes back into the key: a member starting there has for
		// its key the end of this one's, or the colon after it, neither
		// sensitive when this key is not; and scanning a key again for each
		// quote escaped in it would take time quadratic in its length.
		if key := s[loc[2]:loc[3]]; !IsSensitiveKey(key) {
			next := loc[5]
			b.WriteString(s[:next])
			s = s[next:]
			continue
		}
		b.WriteString(s[:loc[5]])
		b.WriteString(quote + Masked)
		s = s[loc[1]-len(quote):]
	}
}

var (
	// urlUserinfo matches the scheme and userinfo of a URL
	// ("scheme://user:pw@"). The userinfo stops at the last "@" before the
	// path, query or fragment, so a password with an unescaped "@" is
	// still covered.
	urlUserinfo = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)([^/?#\s]*)@`)
	// urlPassword matches the scheme, user and password of a URL whose
	// password holds an unescaped "/", "?" or "#" ("scheme://user:p/w@"),
	// up to the first "@" after it.
	urlPassword = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.\-]*://)([^:/?#@\s]*):([^@\s]*?[/?#][^@\s]*?)@`)
	// queryParam matches one "name=value" parameter of a URL query.
	queryParam = regexp.MustCompile(`([?&;])([^=&#;\s]+)=([^&#;\s]*)`)
)

// maskURLs masks the credentials of every URL in s: the password of its
// userinfo, keeping the user name, and query parameters with sensitive
// names ("?api_key=…"), of a URL without a scheme too ("host/health?token=…",
// "user=app&password=…"). A userinfo without a password is masked whole,
// since tokens are often passed as the user name ("https://ghp_…@github.com").
func maskURLs(s string) string {
	if strings.Contains(s, "://") {
		s = urlUserinfo.ReplaceAllStringFunc(s, func(match string) string {
			parts := urlUserinfo.FindStringSubmatch(match)
			scheme, userinfo := parts[1], parts[2]
			if userinfo == "" {
				return match
			}
			if user, _, hasPassword := strings.Cut(userinfo, ":"); hasPassword {
				return scheme + user + ":" + Masked + "@"
			}
			return scheme + Masked + "@"
		})
		s = urlPassword.ReplaceAllStringFunc(s, func(match string) string {
			parts := urlPassword.FindStringSubmatch(match)
			scheme, user, password := parts[1], parts[2], parts[3]
			// "host:443/@scope" is a host, a port and a path.
			if port := password[:strings.IndexAny(password, "/?#")]; port != "" && strings.Trim(port, "0123456789") == "" {
				return match
			}
			return scheme + user + ":" + Masked + "@"
		})
	} else if !strings.ContainsAny(s, "?&") {
		return s
	}
	return queryParam.ReplaceAllStringFunc(s, func(match string) string {
		parts := queryParam.FindStringSubmatch(match)
		if !IsSensitiveKey(parts[2]) {
			return match
		}
		return parts[1] + parts[2] + "=" + Masked
	})
}
