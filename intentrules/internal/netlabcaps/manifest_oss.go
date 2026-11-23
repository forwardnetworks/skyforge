package netlabcaps

import _ "embed"

// This build embeds netlab_capabilities_oss.json: the capability manifest generated
// from upstream netlab with the public patch series applied.

//go:embed netlab_capabilities_oss.json
var manifestJSON []byte
