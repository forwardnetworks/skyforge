package intentrules

import "embed"

// The model is data. schema/ holds stock Infrahub schema YAML and nothing else; sidecar/ holds what Skyforge adds to it; jsonschema/
// holds the published Infrahub JSON schema, used to reject keys Infrahub would reject. Everything is embedded, so the evaluator
// needs no file at run time and no Infrahub.
//
//go:embed schema/*.yml sidecar/*.yml sidecar/rules/*.yml jsonschema/infrahub-schema.json jsonschema/NOTICE jsonschema/LICENSE-Apache-2.0.txt
var embedded embed.FS

// ThirdPartyNotices returns the NOTICE and the Apache-2.0 licence text for the Infrahub JSON schema that is embedded above, so a
// binary that carries the schema also carries the attribution Apache-2.0 asks for.
func ThirdPartyNotices() (notice, licence string, err error) {
	n, err := embedded.ReadFile("jsonschema/NOTICE")
	if err != nil {
		return "", "", err
	}
	l, err := embedded.ReadFile("jsonschema/LICENSE-Apache-2.0.txt")
	if err != nil {
		return "", "", err
	}
	return string(n), string(l), nil
}
