package netlabcapsgen

import "os"

// markersPath is the marker list for a generator run; NETLAB_MARKERS names it. Unset means the tests that need it skip.
func markersPath() string { return os.Getenv("NETLAB_MARKERS") }
