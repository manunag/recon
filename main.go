package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	key := flag.String("key", "id", "field that identifies a record")
	ignore := flag.String("ignore", "", "comma-separated fields to ignore")
	samples := flag.Int("samples", 3, "sample IDs to show per category")
	asJSON := flag.Bool("json", false, "print the report as JSON")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: recon [flags] source.jsonl target.jsonl")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	ign := make(map[string]bool)
	for _, f := range strings.Split(*ignore, ",") {
		if f = strings.TrimSpace(f); f != "" {
			ign[f] = true
		}
	}

	src, dst, err := LoadFiles(flag.Arg(0), flag.Arg(1), *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "recon:", err)
		os.Exit(2)
	}
	rep := Compare(src, dst, Options{Key: *key, Ignore: ign, Samples: *samples})

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rep)
	} else {
		rep.WriteText(os.Stdout)
	}
	if rep.Mismatches() > 0 {
		os.Exit(1)
	}
}
