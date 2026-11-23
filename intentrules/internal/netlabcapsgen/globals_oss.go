package netlabcapsgen

// globalBlockNames returns the top-level blocks the generator requires and reads.
func globalBlockNames(bool) []string { return []string{"otg"} }

// ossBuild makes the generator produce the public manifest whatever its options say.
const ossBuild = true

// manifestSource is the manifest's provenance line.
func manifestSource(bool) string { return ossManifestSource }
