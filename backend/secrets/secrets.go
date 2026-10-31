// Package secrets masks credentials in the configuration DockGraph exposes
// (environment variables, labels, commands, driver options and event
// attributes, of running containers and parsed compose services alike), so
// every code path hides them the same way.
//
// A value under a key that looks like a credential (DB_PASSWORD,
// API_TOKEN, …) is masked whole. Any other value is scanned for credentials
// embedded in it: URL, DSN and JDBC passwords, sensitive query parameters,
// "key=value" pairs and lists, flags, headers, JSON members and XML
// elements. Masking is a best effort: logs, health-check output and free
// text can still carry secrets it does not recognise.
package secrets

// Masked is the placeholder substituted for sensitive values.
const Masked = "********"

// maxValueLen is the length past which a value is masked whole rather than
// scanned, bounding the work one value can cost. Configuration values are
// far shorter, so nothing worth reading is lost.
const maxValueLen = 64 << 10

// MaskEnvValue returns value masked for key: entirely when the key looks
// like a credential, otherwise only the credentials embedded in it. Empty
// and boolean values are masked too: that a password is empty, or that
// authentication is off, is as telling as the password. The key stays
// visible, so a network's "encrypted" option still shows.
func MaskEnvValue(key, value string) string {
	if IsSensitiveKey(key) {
		return Masked
	}
	return MaskValue(value)
}

// MaskLabels returns a copy of a key-value map (labels, driver options,
// event attributes) with every value masked as MaskEnvValue does. Labels
// often carry reverse-proxy settings such as Traefik basic-auth user hashes.
func MaskLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = MaskEnvValue(k, v)
	}
	return out
}

// MaskCommand returns copies of a container's entrypoint and command with
// credentials masked, read together as the one command line Docker runs, so
// a flag ending the entrypoint ("--requirepass") has its value in the
// command masked. An argument reaches the program whole, so a value in it
// goes on to the end of the argument, spaces included; the command a shell
// runs ("sh -c '…'") is shell text, masked as MaskValue does. An argument
// longer than maxValueLen is masked whole, and so is every argument after
// it.
func MaskCommand(entrypoint, cmd []string) (maskedEntrypoint, maskedCmd []string) {
	var m masker
	return m.args(entrypoint), m.args(cmd)
}

// MaskValue masks the credentials embedded in free text (an environment or
// label value, health-check output), read as shell text: a value ends at
// whitespace unless it is quoted. A value longer than maxValueLen is masked
// whole.
func MaskValue(s string) string {
	if len(s) > maxValueLen {
		return Masked
	}
	var m masker
	return m.text(s, false)
}
