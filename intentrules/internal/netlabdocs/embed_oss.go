package netlabdocs

import "embed"

// This build embeds corpus_oss/ and corpus_index_oss.json: netlab's documentation
// for the public netlab patch series, with the index that describes it.

const corpusRoot = "corpus_oss"

//go:embed corpus_oss
var corpusFS embed.FS

//go:embed corpus_index_oss.json
var indexJSON []byte
