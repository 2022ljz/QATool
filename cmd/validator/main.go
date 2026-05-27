package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"qatool/internal/validator"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "check" {
		fmt.Fprintln(os.Stderr, "usage: validator check [--schema path] [--rules path] [--presets path] --check path [--data-dir dir] [--out path]")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("check", flag.ExitOnError)
	opts := validator.Options{}
	fs.StringVar(&opts.SchemaPath, "schema", "layer/01_schema.yaml", "schema yaml path")
	fs.StringVar(&opts.RulesPath, "rules", "layer/02_rule_library.yaml", "rule library yaml path")
	fs.StringVar(&opts.PresetsPath, "presets", "layer/03_presets.yaml", "presets yaml path")
	fs.StringVar(&opts.CheckPath, "check", "layer/04_checks_summer_night.yaml", "checks yaml path")
	fs.StringVar(&opts.CheckPath, "c", "layer/04_checks_summer_night.yaml", "checks yaml path")
	fs.StringVar(&opts.DataDir, "data-dir", "", "csv data directory")
	fs.StringVar(&opts.OutPath, "out", "", "report output path")
	fs.IntVar(&opts.Workers, "workers", 4, "rule worker count")
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	reportPath, err := validator.Run(context.Background(), opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "validator:", err)
		os.Exit(1)
	}
	fmt.Println("report written:", reportPath)
}
