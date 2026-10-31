package api

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dockgraph/dockgraph/secrets"
)

// Agents mask their answers as the server masks its own, and the server
// masks the credential fields of an agent's answer again, so their masking
// does not depend on the version an agent runs. Masking a masked answer
// again can hide a little more, never less.

// errNotAnObject reports an agent answer that is not a JSON object.
var errNotAnObject = errors.New("not a JSON object")

// remaskInspect masks the credentials in an agent's container inspect
// answer, the JSON built by buildInspectResponse. Fields the answer lacks
// stay absent; other fields pass as they are.
func remaskInspect(body []byte) ([]byte, error) {
	fields, err := decodeObject(body)
	if err != nil {
		return nil, err
	}

	if err := maskField(fields, fieldEnv, func(env []map[string]string) []map[string]string {
		for _, e := range env {
			if e != nil {
				e["value"] = secrets.MaskEnvValue(e["key"], e["value"])
			}
		}
		return env
	}); err != nil {
		return nil, err
	}

	if err := maskField(fields, fieldLabels, secrets.MaskLabels); err != nil {
		return nil, err
	}

	var entrypoint, cmd []string
	if err := decodeField(fields, fieldEntrypoint, &entrypoint); err != nil {
		return nil, err
	}
	if err := decodeField(fields, fieldCmd, &cmd); err != nil {
		return nil, err
	}
	entrypoint, cmd = secrets.MaskCommand(entrypoint, cmd)
	if err := setField(fields, fieldEntrypoint, entrypoint); err != nil {
		return nil, err
	}
	if err := setField(fields, fieldCmd, cmd); err != nil {
		return nil, err
	}

	if raw, ok := fields[fieldHealth]; ok && string(raw) != "null" {
		health, err := remaskHealth(raw)
		if err != nil {
			return nil, fmt.Errorf("health: %w", err)
		}
		fields[fieldHealth] = health
	}
	return json.Marshal(fields)
}

// remaskHealth masks the probe output in a health object (see buildHealth).
func remaskHealth(raw json.RawMessage) (json.RawMessage, error) {
	health, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	if err := maskField(health, "log", func(entries []map[string]json.RawMessage) []map[string]json.RawMessage {
		for _, e := range entries {
			var output string
			if json.Unmarshal(e["output"], &output) == nil {
				e["output"], _ = json.Marshal(secrets.MaskValue(output))
			} else {
				delete(e, "output") // not a string: nothing the browser can use
			}
		}
		return entries
	}); err != nil {
		return nil, err
	}
	return json.Marshal(health)
}

// remaskVolume masks the credentials in an agent's volume inspect answer,
// the JSON built by buildVolumeResponse.
func remaskVolume(body []byte) (map[string]json.RawMessage, error) {
	fields, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{fieldLabels, fieldOptions} {
		if err := maskField(fields, name, secrets.MaskLabels); err != nil {
			return nil, err
		}
	}
	if err := maskField(fields, fieldStatus, maskDriverStatus); err != nil {
		return nil, err
	}
	return fields, nil
}

// maskDriverStatus masks a volume driver's status like labels. Drivers put
// free-form details there, such as the source a volume was mounted from or
// the arguments of a mount helper, at any depth.
func maskDriverStatus(status map[string]any) map[string]any {
	if status == nil {
		return nil
	}
	masked := make(map[string]any, len(status))
	for k, v := range status {
		masked[k] = maskStatusValue(k, v)
	}
	return masked
}

// maskStatusValue masks the status value v found under key: a string like an
// environment value, a map member by member, a list's strings together like
// command arguments (so a flag masks the value after it) and its other
// members one by one, and anything under a sensitive key whole.
func maskStatusValue(key string, v any) any {
	if s, ok := v.(string); ok {
		return secrets.MaskEnvValue(key, s)
	}
	if secrets.IsSensitiveKey(key) {
		return secrets.Masked
	}
	switch v := v.(type) {
	case map[string]any:
		return maskDriverStatus(v)
	case []any:
		out := make([]any, len(v))
		var args []string
		var at []int
		for i, e := range v {
			if s, ok := e.(string); ok {
				args = append(args, s)
				at = append(at, i)
				continue
			}
			out[i] = maskStatusValue(key, e)
		}
		_, masked := secrets.MaskCommand(nil, args)
		for j, i := range at {
			out[i] = masked[j]
		}
		return out
	}
	return v
}

// decodeObject decodes a JSON object into its raw fields.
func decodeObject(body []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errNotAnObject
	}
	return fields, nil
}

// decodeField decodes field name into v, leaving v untouched when the field
// is absent or null.
func decodeField(fields map[string]json.RawMessage, name string, v any) error {
	raw, ok := fields[name]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// setField replaces field name with v, if the answer has the field.
func setField(fields map[string]json.RawMessage, name string, v any) error {
	if _, ok := fields[name]; !ok {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fields[name] = raw
	return nil
}

// maskField replaces field name, when present and not null, with mask
// applied to its decoded value.
func maskField[T any](fields map[string]json.RawMessage, name string, mask func(T) T) error {
	raw, ok := fields[name]
	if !ok || string(raw) == "null" {
		return nil
	}
	var v T
	if err := decodeField(fields, name, &v); err != nil {
		return err
	}
	return setField(fields, name, mask(v))
}
