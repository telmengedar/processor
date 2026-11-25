package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/telmengedar/processor/internal/boot"
	"github.com/telmengedar/processor/internal/divoid"
	"github.com/telmengedar/processor/internal/process"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

func run(args []string, input io.Reader, output, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("process-import", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	example := flags.String("example", "", "gangolf-chat or python")
	personaType := flags.String("persona-type", "", "observed persona node type (required for v2 example)")
	printOnly := flags.Bool("print", false, "print blueprint without publishing")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	var blueprint process.Blueprint
	switch *example {
	case "gangolf-chat":
		var err error
		blueprint, err = process.GangolfV2(*personaType)
		if err != nil {
			fmt.Fprintln(diagnostic, "explicit persona node type required or example unavailable")
			return 2
		}
	case "python":
		blueprint = process.PythonExample()
	case "":
		d := json.NewDecoder(io.LimitReader(input, process.MaxDefinitionBytes+1))
		d.DisallowUnknownFields()
		d.UseNumber()
		if d.Decode(&blueprint) != nil || d.Decode(new(any)) != io.EOF {
			fmt.Fprintln(diagnostic, "invalid blueprint JSON")
			return 2
		}
	default:
		fmt.Fprintln(diagnostic, "unknown example")
		return 2
	}
	if *printOnly {
		if json.NewEncoder(output).Encode(blueprint) != nil {
			return 1
		}
		return 0
	}
	cfg, err := boot.LoadGraph()
	if err != nil {
		fmt.Fprintln(diagnostic, "Graph configuration unavailable")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	publication, err := process.Import(ctx, divoid.NewClient(cfg.URL, cfg.Key, nil, nil), blueprint)
	if err != nil {
		_ = json.NewEncoder(output).Encode(publication)
		fmt.Fprintln(diagnostic, "publication incomplete; retain returned allocated IDs for owner cleanup")
		return 1
	}
	if json.NewEncoder(output).Encode(publication) != nil {
		return 1
	}
	return 0
}
