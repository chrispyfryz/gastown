package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// decodeBDJSON decodes JSON from bd output, tolerating leading non-JSON lines.
//
// bd sometimes prints warnings (e.g. --allow-stale staleness banner) to stdout
// even when --json is requested. This helper finds the first JSON token and
// decodes from there.
func decodeBDJSON(out []byte, v any) error {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return fmt.Errorf("empty output")
	}

	// Find first JSON container start.
	idx := bytes.IndexAny(out, "[{")
	if idx < 0 {
		return fmt.Errorf("no JSON found in output")
	}

	dec := json.NewDecoder(bytes.NewReader(out[idx:]))
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}
