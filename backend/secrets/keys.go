package secrets

import (
	"slices"
	"strings"
)

// sensitiveKeyPatterns matches keys whose values should be masked. Matching
// is by substring, case-insensitive: "PASS" covers PASSWORD, PASSWD,
// SMTP_PASS and PASSPHRASE.
var sensitiveKeyPatterns = []string{
	"PASS", "PWD", "SECRET", "KEY", "TOKEN", "CREDENTIAL", "CREDS",
	"API_KEY", "APIKEY", "PRIVATE", "AUTH", "CERT",
	"SSL_", "TLS_", "ENCRYPT", "ACCESS_KEY", "SESSION",
	"DSN", "CONNECTION_STRING", "CONNECTIONSTRING", "CONN_STR", "COOKIE", "SALT", "WEBHOOK",
}

// notSensitiveKeys are keys a pattern matches that hold no secret: the
// shell's working directories.
var notSensitiveKeys = map[string]bool{"PWD": true, "OLDPWD": true}

// benignWords contain a pattern without being about credentials
// (org.opencontainers.image.authors, PASSENGER_APP_ENV, FTP_PASSIVE). They
// are removed from a key before matching, so "AUTHOR_TOKEN" still matches.
var benignWords = strings.NewReplacer(
	"AUTHOR", "", "PASSENGER", "", "PASSIVE", "", "PASSTHROUGH", "", "BYPASS", "", "COMPASS", "",
)

// credentialWords start with a benign word but name credentials
// (AUTHORIZATION, AUTHORISATION), so they are matched before benign words
// are removed.
var credentialWords = []string{"AUTHORIZ", "AUTHORIS"}

// IsSensitiveKey reports whether a key (an environment variable, label or
// option name) looks like a credential, so its value must be masked before
// leaving the server.
func IsSensitiveKey(key string) bool {
	upper := strings.ToUpper(key)
	if notSensitiveKeys[upper] {
		return false
	}
	if namesAuthorization(upper) {
		return true
	}
	upper = benignWords.Replace(upper)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// namesAuthorization reports whether key names an Authorization header,
// whose value is a scheme and, after a space, the credentials.
func namesAuthorization(key string) bool {
	upper := strings.ToUpper(key)
	for _, word := range credentialWords {
		if strings.Contains(upper, word) {
			return true
		}
	}
	return false
}

// credentialNames are the words that name a credential at the end of a key
// joined to its value by a colon ("db_password:x"). Only the key's last word
// is matched, rather than any part of it as IsSensitiveKey matches, since
// images and hosts are written the same way ("keycloak:22").
var credentialNames = []string{
	"PASSWORD", "PASSWD", "PASS", "PASSPHRASE", "PWD", "SECRET", "TOKEN", "KEY", "APIKEY",
	"AUTH", "AUTHORIZATION", "CREDENTIAL", "CREDENTIALS", "COOKIE",
}

// namesCredential reports whether key, joined to a value by a colon, names
// a credential: whether its last word is one of credentialNames.
func namesCredential(key string) bool {
	if notSensitiveKeys[strings.ToUpper(key)] {
		return false
	}
	last := key[strings.LastIndexAny(key, "_.-")+1:]
	return slices.Contains(credentialNames, strings.ToUpper(last))
}
