// Command intentrules drives the design-time rules layer (internal/intentrules) from a shell and from CI.
//
//	intentrules inspect [-v] FILE...                 run the rules over topology files; one line per finding
//	intentrules docs                                 print the generated rules page (docs/intent-rules.md)
//	intentrules schemacheck -schema DIR -jsonschema FILE   validate every schema file in DIR against Infrahub's published JSON schema
//
// Exit status: 0 ok, 1 findings in schemacheck or a failure, 3 cannot evaluate. inspect exits 0 whatever it finds: the caller
// decides what is allowed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/intentrules"
	"github.com/google/jsonschema-go/jsonschema"
	"sigs.k8s.io/yaml"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: intentrules inspect|docs|schemacheck ...")
		os.Exit(3)
	}
	switch os.Args[1] {
	case "inspect":
		os.Exit(inspect(os.Args[2:]))
	case "docs":
		e, err := intentrules.Default()
		if err != nil {
			fmt.Fprintln(os.Stderr, "intentrules: the embedded model does not load:", err)
			os.Exit(3)
		}
		fmt.Print(intentrules.RenderDocs(e))
	case "schemacheck":
		os.Exit(schemacheck(os.Args[2:]))
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(3)
	}
}

func inspect(args []string) int {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	verbose := fs.Bool("v", false, "also print what was evaluated and what was skipped")
	_ = fs.Parse(args)
	e, err := intentrules.Default()
	if err != nil {
		fmt.Fprintln(os.Stderr, "intentrules: the embedded model does not load:", err)
		return 3
	}
	for _, f := range fs.Args() {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 3
		}
		res := e.InspectYAML(string(b), intentrules.Options{})
		for _, fd := range res.Findings {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", f, fd.Class, fd.Code, fd.Path, strings.Join(fd.Subject, ","), strings.ReplaceAll(fd.Message, "\n", " "))
		}
		if *verbose {
			keys := make([]string, 0, len(res.Skipped))
			for k := range res.Skipped {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s\tSKIPPED\t%s\t%s\n", f, k, res.Skipped[k])
			}
			fmt.Printf("%s\tEVALUATED\t%s\n", f, strings.Join(res.Evaluated, ","))
		}
	}
	return 0
}

// schemacheck validates each .yml/.yaml/.json under the schema directory against the published Infrahub JSON schema. That schema
// has additionalProperties false everywhere, so an extension key in the schema directory fails here. No Python is involved.
func schemacheck(args []string) int {
	fs := flag.NewFlagSet("schemacheck", flag.ExitOnError)
	dir := fs.String("schema", "", "directory of Infrahub schema files")
	js := fs.String("jsonschema", "", "the published Infrahub JSON schema")
	_ = fs.Parse(args)
	if *dir == "" || *js == "" {
		fmt.Fprintln(os.Stderr, "schemacheck needs -schema and -jsonschema")
		return 3
	}
	raw, err := os.ReadFile(*js)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read the JSON schema:", err)
		return 3
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		fmt.Fprintln(os.Stderr, "the JSON schema does not parse:", err)
		return 3
	}
	resolved, err := s.Resolve(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "the JSON schema does not resolve:", err)
		return 3
	}
	var files []string
	_ = filepath.WalkDir(*dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			switch strings.ToLower(filepath.Ext(p)) {
			case ".yml", ".yaml", ".json":
				files = append(files, p)
			}
		}
		return nil
	})
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no schema files under %s: a check over nothing proves nothing\n", *dir)
		return 3
	}
	bad := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 3
		}
		j, err := yaml.YAMLToJSON(b)
		if err != nil {
			fmt.Printf("FAIL %s: not YAML: %v\n", f, err)
			bad++
			continue
		}
		var doc any
		if err := json.Unmarshal(j, &doc); err != nil {
			fmt.Printf("FAIL %s: %v\n", f, err)
			bad++
			continue
		}
		if err := resolved.Validate(doc); err != nil {
			fmt.Printf("FAIL %s: %v\n", f, err)
			bad++
			continue
		}
		fmt.Printf("ok   %s\n", f)
	}
	if bad > 0 {
		return 1
	}
	return 0
}
