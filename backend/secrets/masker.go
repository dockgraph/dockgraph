package secrets

import (
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// word matches a run of characters other than whitespace and NUL, which
// separates the variables of an environment block (/proc/<pid>/environ).
var word = regexp.MustCompile(`[^\s\x00]+`)

// spaces matches the spaces a shell does not split words on: the no-break
// space and the other Unicode spaces beyond ASCII whitespace.
var spaces = regexp.MustCompile(`\p{Z}+`)

// expectation is what a word announces about the one after it.
type expectation int

const (
	expectAnything expectation = iota
	// expectValue follows a sensitive flag ("--password").
	expectValue
	// expectHeaderValue follows a sensitive header or key name
	// ("Authorization:"); an authentication scheme ("Bearer") may come
	// before the value.
	expectHeaderValue
	// expectUserinfo follows a user flag ("-u") whose argument may carry a
	// password ("user:password").
	expectUserinfo
	// expectEquals follows a sensitive key standing alone, which an "=" and
	// the value may follow ("password = …", as TOML and INI files write it).
	expectEquals
)

// masker masks a sequence of words, remembering across words (and across
// arguments) what the previous one announced, and across the words of a
// quoted value.
type masker struct {
	expect expectation
	// quote is the quote a masked value opened without closing it: the value
	// goes on into the following words, up to that quote.
	quote byte
	// maskedValue tells that the last word held a value, now masked.
	maskedValue bool
	// connValue tells that a masked value of a connection string goes on
	// into the following words, up to the semicolon ending it.
	connValue bool
	// program is the name of the last argument that is no option: the
	// program the options after it are given to ("busybox sh -c").
	program string
	// optionValue tells that the next argument is the value of a shell
	// option ("-o pipefail").
	optionValue bool
	// shellCommand tells that the next argument is the command a shell runs.
	shellCommand bool
	// unread tells that an argument was too long to scan. What it announced
	// is unknown, and what it announced decides how the arguments after it
	// read, so they are all masked.
	unread bool
}

// shellCommandFlag matches the option after which a shell takes the command
// to run: "-c", alone or ending other options ("sh -ec", "bash -lc").
var shellCommandFlag = regexp.MustCompile(`^-[a-z]*c$`)

// shellOptionWithValue matches a shell option whose value is the next
// argument: "-o" or bash's "-O", alone or ending other options ("bash -euo
// pipefail"), both also with "+", which turns the option off, and bash's
// "--rcfile" and "--init-file".
var shellOptionWithValue = regexp.MustCompile(`^([-+][A-Za-z]*[oO]|--rcfile|--init-file)$`)

// shells are the programs whose "-c" takes a command. Other programs use the
// option for something else (postgres sets a parameter with it), and their
// argument after it is a value like any other.
var shells = []string{"sh", "bash", "dash", "ash", "zsh", "ksh", "mksh", "busybox"}

// args masks every argument of a command line: each one as a single value,
// except the command a shell runs, which is shell text.
func (m *masker) args(args []string) []string {
	if args == nil {
		return nil
	}
	out := make([]string, len(args))
	for i, a := range args {
		shellText := m.shellCommand
		m.shellCommand = shellCommandFlag.MatchString(a) && slices.Contains(shells, m.program)
		// An option, or a shell option's value ("-o pipefail"), names no
		// program.
		names := !strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "+") && !m.optionValue
		m.optionValue = shellOptionWithValue.MatchString(a)
		if m.unread || len(a) > maxValueLen {
			m.unread = true
			out[i] = Masked
			continue
		}
		out[i] = m.text(a, !shellText)
		// Nor does an argument holding a mask, such as a flag's value
		// ("--password sh"): masked again, it would name another program
		// than the first time.
		if names && !strings.Contains(out[i], Masked) {
			m.program = path.Base(a)
		}
	}
	return out
}

// text masks every word of s, keeping the whitespace between them, then the
// JSON members and XML elements left. They come second: matched first, a
// string spanning several words could hide a word that announces a value.
// When s is an argument (arg) rather than shell text, a value goes on to its
// end: the program gets the argument whole, so neither spaces nor quotes end
// a value.
func (m *masker) text(s string, arg bool) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	prev := 0
	inValue := false
	for _, loc := range word.FindAllStringIndex(s, -1) {
		b.WriteString(s[prev:loc[0]])
		m.maskedValue = false
		// Every word is read, even inside a value, for what it announces
		// about the next argument ("--db-password=x --token").
		masked := m.parts(s[loc[0]:loc[1]])
		if inValue {
			masked = maskInValue(masked)
		}
		b.WriteString(masked)
		inValue = inValue || arg && m.maskedValue
		prev = loc[1]
	}
	b.WriteString(s[prev:])
	// The quotes in an argument of an exec-form command are plain
	// characters, so a quoted value never goes on into the next argument,
	// nor does the value of a connection string.
	m.quote = 0
	m.connValue = false
	return maskXMLElements(maskJSONMembers(b.String()))
}

// parts masks w, a word, as word does, unless spaces other than ASCII
// whitespace split it ("app --password x"): they read as spaces,
// so each part is masked as a word, but a shell takes the word whole, so a
// value one part begins goes on to the word's end.
func (m *masker) parts(w string) string {
	if isASCII(w) {
		return m.word(w)
	}
	seps := spaces.FindAllStringIndex(w, -1)
	if seps == nil {
		return m.word(w)
	}
	var b strings.Builder
	prev, inValue, maskedValue := 0, false, false
	for _, sep := range append(seps, []int{len(w), len(w)}) {
		if part := w[prev:sep[0]]; part != "" {
			m.maskedValue = false
			masked := m.word(part)
			if inValue {
				masked = maskInValue(masked)
			}
			b.WriteString(masked)
			inValue = inValue || m.maskedValue
			maskedValue = maskedValue || m.maskedValue
		}
		b.WriteString(w[sep[0]:sep[1]])
		prev = sep[1]
	}
	m.maskedValue = maskedValue
	return b.String()
}

// isASCII reports whether s holds ASCII characters only.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// word masks one whitespace-free word.
func (m *masker) word(w string) string {
	if m.quote != 0 {
		return m.continueQuoted(w)
	}
	if m.connValue {
		// A connection string's value ends at a semicolon, not a space.
		m.connValue = !strings.Contains(w, ";")
		m.maskedValue = true
		return Masked + trailingQuotes(w)
	}
	expected := m.expect
	m.expect = expectAnything
	// "--password --verbose": the flag took no value after all.
	if expected != expectAnything && !strings.HasPrefix(w, "-") {
		switch expected {
		case expectValue:
			return m.maskWord(w, false)
		case expectHeaderValue:
			if isAuthScheme(w) {
				m.expect = expectValue
				return w
			}
			return m.maskWord(w, false)
		case expectUserinfo:
			if hasUserPassword(w) {
				return m.maskUserinfo(w)
			}
		case expectEquals:
			if w == "=" {
				m.expect = expectValue
				return w
			}
			if value, ok := strings.CutPrefix(w, "="); ok {
				return "=" + m.maskWord(value, false)
			}
		}
	}
	w = maskDSNs(maskURLs(w))
	if name, isFlag := flagName(unquote(w)); isFlag && !strings.Contains(w, "=") {
		switch {
		case isUserFlag(name):
			m.expect = expectUserinfo
		// A one-letter flag ("-p") is too ambiguous to mask what follows.
		case len(name) > 1 && IsSensitiveKey(name):
			m.expect = expectValue
		}
		return w
	}
	if headerKey.MatchString(unquote(w)) {
		if IsSensitiveKey(strings.TrimSuffix(unquote(w), ":")) {
			m.expect = expectHeaderValue
		}
		return w
	}
	if masked, ok := m.maskKeyValue(w); ok {
		return masked
	}
	if masked, ok := m.maskPair(w, false); ok {
		return masked
	}
	if keyName.MatchString(w) && IsSensitiveKey(w) {
		m.expect = expectEquals
		return w
	}
	return m.maskPairList(w)
}

// keyName matches a key standing alone: an identifier.
var keyName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*$`)

// maskKeyValue masks the value of w when it joins a credential's name and
// its value with a colon and no space ("password:x", "X-Api-Key:x"). The
// value of "Authorization:Bearer" is the next word. ok is false when w is no
// such word.
func (m *masker) maskKeyValue(w string) (masked string, ok bool) {
	u := unquote(w)
	key, value, found := strings.Cut(u, ":")
	if !found || value == "" || !keyName.MatchString(key) || !namesCredential(key) {
		return w, false
	}
	if isAuthScheme(value) {
		m.expect = expectValue
		return w, true
	}
	return w[:len(w)-len(u)] + key + ":" + m.maskWord(value, false), true
}

// headerKey matches a word naming a header or a key followed by its value
// ("Authorization: Bearer …", "password: …").
var headerKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*:$`)

// authSchemes are the HTTP authentication schemes that come before the
// credentials in an Authorization header. ApiKey and Api-Token are what
// Elasticsearch and Dynatrace use.
var authSchemes = []string{"Basic", "Bearer", "Digest", "Negotiate", "Token", "ApiKey", "Api-Token"}

// isAuthScheme reports whether w is an HTTP authentication scheme.
func isAuthScheme(w string) bool {
	w = unquote(w)
	for _, scheme := range authSchemes {
		if strings.EqualFold(w, scheme) {
			return true
		}
	}
	return false
}

// isUserFlag reports whether a flag name gives a user that may carry a
// password, as curl's "-u user:password" does.
func isUserFlag(name string) bool {
	return name == "u" || name == "user"
}

// pairKey matches the key of a "key=value" pair or "--flag=value" argument:
// optional dashes and an identifier, which a URL or path never is.
var pairKey = regexp.MustCompile(`^-{0,2}[A-Za-z_][A-Za-z0-9_.\-]*$`)

// maskPair masks the value of w when it is a "key=value" pair with a
// sensitive key, or holds one as its value ("--env=DB_PASSWORD=x",
// "--set=auth.password=x"). ok is false when w is no such pair. In a
// connection string (connString), "PWD" is the password rather than the
// shell's working directory, and braces quote a value. A value that is an
// authentication scheme, or any value of an Authorization key, has the
// credentials in the next word ("Authorization=Bearer x"), so that word is
// masked too; a quoted value already covers the words up to its quote, and a
// connection string's value the words up to its semicolon.
func (m *masker) maskPair(w string, connString bool) (masked string, ok bool) {
	// start is where the pair read begins: past the keys holding it.
	for start := 0; ; {
		key, value, found := strings.Cut(w[start:], "=")
		if !found || !pairKey.MatchString(unquote(key)) {
			return w, false
		}
		head := start + len(key) + 1 // the keys and their "="
		name, isFlag := flagName(unquote(key))
		switch {
		case IsSensitiveKey(name), connString && strings.EqualFold(name, "PWD"):
			masked := w[:head] + m.maskWord(value, connString)
			if !connString && m.quote == 0 && (isAuthScheme(value) || namesAuthorization(name)) {
				m.expect = expectValue
			}
			return masked, true
		case isFlag && isUserFlag(name) && hasUserPassword(value):
			return w[:head] + m.maskUserinfo(value), true
		}
		start = head
	}
}

// unquote drops the shell quotes that open a word inside a quoted command
// ("sh -c 'PGPASSWORD=…").
func unquote(w string) string {
	return strings.TrimLeft(w, `'"`)
}

// maskPairList masks a "key=value" list, separated by commas as in mount
// options ("addr=10.0.0.5,username=app,password=pw") or by semicolons as in
// connection strings ("Server=db;Password=pw") and command chains. From the
// first item with a sensitive key on, the rest of the word is masked: what
// follows a separator may still belong to its value ("password=pa;ss").
func (m *masker) maskPairList(w string) string {
	if !strings.ContainsAny(w, ",;") || !strings.Contains(w, "=") {
		return w
	}
	connString := isConnectionString(w)
	var b strings.Builder
	for {
		end := strings.IndexAny(w, ",;")
		// Only an item holding "=" can be a pair. For any other, maskPair
		// would look for one in the items after it, scanning the rest of the
		// list again for every such item.
		item := w
		if end >= 0 {
			item = w[:end]
		}
		if strings.Contains(item, "=") {
			if masked, ok := m.maskPair(w, connString); ok {
				// A connection string's value goes on to a semicolon, spaces
				// included ("Password=pa ss;"): past this word, when it does
				// not end here.
				m.connValue = connString && m.quote == 0 && !strings.Contains(w[strings.IndexByte(w, '=')+1:], ";")
				b.WriteString(masked)
				return b.String()
			}
		}
		if end < 0 {
			b.WriteString(w)
			return b.String()
		}
		b.WriteString(w[:end+1])
		w = w[end+1:]
	}
}

// connectionStringKeys are keys of a connection string that a shell
// command or a mount option list has no use for.
var connectionStringKeys = []string{"SERVER", "UID", "DRIVER", "DATABASE"}

// isConnectionString reports whether the "key=value" list w has an item
// with a connection string key.
func isConnectionString(w string) bool {
	for _, item := range strings.FieldsFunc(w, func(r rune) bool { return r == ',' || r == ';' }) {
		key, _, found := strings.Cut(item, "=")
		if found && slices.Contains(connectionStringKeys, strings.ToUpper(unquote(key))) {
			return true
		}
	}
	return false
}

// closingQuote returns the character closing a value that opens with c: a
// shell quote or, with braces (in a connection string), a brace; 0 when c
// opens no quoted value.
func closingQuote(c byte, braces bool) byte {
	switch {
	case c == '\'', c == '"':
		return c
	case c == '{' && braces:
		return '}'
	}
	return 0
}

// maskWord returns the mask of a value word, keeping the quotes around it
// (braces too, in a connection string). A value that opens a quote without
// closing it goes on into the following words, up to the closing quote;
// what follows that quote is dropped too, since it may belong to the value
// ('pw'x is pwx to a shell).
func (m *masker) maskWord(w string, braces bool) string {
	m.maskedValue = true
	if w == "" || closingQuote(w[0], braces) == 0 {
		return Masked + trailingQuotes(w)
	}
	closing := closingQuote(w[0], braces)
	end := strings.IndexByte(w[1:], closing)
	if end < 0 {
		m.quote = closing
		return w[:1] + Masked
	}
	return w[:1] + Masked + string(closing) + trailingQuotes(w[end+2:])
}

// valuePunctuation is what masking a word inside a value keeps: the quotes
// and backslashes that may close or escape a JSON string the value is part
// of, the rest of JSON's punctuation, and "=", without which a masked
// argument holding a pair could read as a flag when masked again.
const valuePunctuation = `"'\:,{}[]=`

// maskInValue masks a word that goes on a value begun by an earlier word of
// the same argument, turning every run of characters but valuePunctuation
// into Masked. A JSON string holding the value then still ends where it did,
// so its member is found and masked whole, words before the value included.
// Leading dashes are kept too: masked again, a word starting with one is
// still no value for a flag before it.
func maskInValue(w string) string {
	var b strings.Builder
	dashes := len(w) - len(strings.TrimLeft(w, "-"))
	b.WriteString(w[:dashes])
	inRun := false
	for i := dashes; i < len(w); i++ {
		switch {
		case strings.IndexByte(valuePunctuation, w[i]) >= 0:
			b.WriteByte(w[i])
			inRun = false
		case !inRun:
			b.WriteString(Masked)
			inRun = true
		}
	}
	return b.String()
}

// continueQuoted masks a word inside a quoted value, up to its closing quote.
func (m *masker) continueQuoted(w string) string {
	end := strings.IndexByte(w, m.quote)
	if end < 0 {
		return Masked
	}
	closing := m.quote
	m.quote = 0
	return Masked + string(closing) + trailingQuotes(w[end+1:])
}

// trailingQuotes returns the quotes that end w, closing a quoted string the
// word is the last of ("sh -c '… --token abc'").
func trailingQuotes(w string) string {
	return w[len(strings.TrimRight(w, `'"`)):]
}

// userPassword matches a "user:password" argument, optionally quoted: a user
// name, a colon and a password. A user flag's argument is often something
// else ("date -u +%H:%M", "-u redis://…"), which it does not match.
var userPassword = regexp.MustCompile(`^['"]?[A-Za-z0-9_][A-Za-z0-9_.@\-]*:[^/'"]`)

// hasUserPassword reports whether w is a "user:password" argument with a
// password, rather than a user name alone ("mysql -u root"), a numeric user
// and group ("docker run -u 1000:1000") or another kind of argument.
func hasUserPassword(w string) bool {
	if !userPassword.MatchString(w) {
		return false
	}
	user, _, _ := strings.Cut(unquote(w), ":")
	return strings.Trim(user, "0123456789") != ""
}

// maskUserinfo masks the password of a "user:password" argument (see
// hasUserPassword), keeping the quotes around it. A quoted argument whose
// quote does not close in this word goes on into the following ones.
func (m *masker) maskUserinfo(w string) string {
	m.maskedValue = true
	user, password, _ := strings.Cut(w, ":")
	if quote := closingQuote(w[0], false); quote != 0 && !strings.Contains(password, string(quote)) {
		m.quote = quote
		return user + ":" + Masked
	}
	return user + ":" + Masked + trailingQuotes(w)
}

// flagName returns w without its leading dashes, and whether there were any.
func flagName(w string) (string, bool) {
	name := strings.TrimLeft(w, "-")
	return name, name != w
}
