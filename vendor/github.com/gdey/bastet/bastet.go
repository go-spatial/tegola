package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/template"
)

/*
Usage:
   bastet template.tpl name=value name=value name=value
   bastet -o stuff template.tpl  name=value name=value name=value
*/

var outputFilename string

func init() {
	flag.StringVar(&outputFilename, "output", "", "File to output to, stdout is default")
	flag.StringVar(&outputFilename, "o", "", "File to output to, stdout is default")
}

func usage() string {
	return fmt.Sprintf(`
Usage:
	%v [-o output.txt] template.tpl [name=value ...]
`,
		os.Args[0],
	)

}

func processArgs() (string, map[string]string) {
	vals := make(map[string]string)
	args := flag.Args()
	if len(args) == 1 {
		return args[0], vals
	}
	for _, a := range args[1:] {
		a := strings.TrimSpace(a)
		if a == "" {
			continue
		}
		parts := strings.SplitN(a, "=", 2)
		if len(parts) == 1 {
			key := strings.Replace(a, " ", "_", -1)
			vals[key] = ""
			continue
		}
		key := strings.Replace(parts[0], " ", "_", -1)
		vals[key] = parts[1]
	}
	return args[0], vals
}

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Need a template file to process.")
		fmt.Fprintln(os.Stderr, usage())
		os.Exit(1)
	}
	templatefn, vals := processArgs()
	t, err := template.ParseFiles(templatefn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing template file %v.\n", templatefn)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	out := os.Stdout
	if outputFilename != "" {
		f, err := os.Create(outputFilename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file %v for writing.\n", outputFilename)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer f.Close()
		out = f
	}

	if err := t.Execute(out, vals); err != nil {
		fmt.Fprintf(os.Stderr, "Error running template %v.\n", outputFilename)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}

}
