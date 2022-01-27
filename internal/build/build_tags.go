//go:build ignore
// +build ignore

// This program will generate a set of files that encode which build tags
// were used to build the program, this way this information is available
// to the program at run time.
//
// The program, will iterate through the source directory figuring out which
// tags are used, and for a subset of those tags generate a few files that will
// make use of those tags to fill in a Tags array.
//
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

// from REF: https://stackoverflow.com/questions/56616196/how-to-convert-camel-case-string-to-snake-case
var (
	matchFirstCap = regexp.MustCompile("(.)([[:upper:]][[:lower:]]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([[:upper:]])")
)

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

var (
	fset         = token.NewFileSet()
	TagVar       = flag.String("var", "Tags", "The name of the variable to append tags to.")
	PackageName  = flag.String("package", "github.com/go-spatial/tegola/internal/build", "The package the name to build the file for.")
	KeepOldFiles = flag.Bool("keepFiles", false, "Don't remove the old `generate.go` files.")
	RunCommand   = flag.String("runCommand", "", "The text for the command used to generate files; will default to name of command with args")
	SrcDir       = flag.String("source", "", "This is the source directory, if not given assume the first argument to be the source, or '.'")
	verbose      = flag.Bool("v", false, "turn on extra output")
)

// getPackages will use `go list` and the source directory to
// obtain the list of packages in the directory.
func getPackages(dir string) (pkgs []string, err error) {

	list := exec.Command("go", "list", "./...")
	list.Dir = dir
	stdout, err := list.StdoutPipe()
	if err != nil {
		panic(err)
		return nil, err
	}
	errChan := make(chan error, 1)

	scanner := bufio.NewScanner(stdout)
	go func() {
		errChan <- list.Run()
	}()
	for scanner.Scan() {
		pkgs = append(pkgs, scanner.Text())
	}

	// wait for the list.Run to finish
	err = <-errChan
	if err != nil {
		return nil, err
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return pkgs, nil
}

type getTagsOptions struct {
	// IgnoreCommands will ignore command packages if set to true
	IgnoreCommands bool
	// IgnorePackages is a list of packages to ignored
	IgnorePackages []string
	// IgnoreTags is a list of tag regular expressions that should be used to ignore tags
	IgnoreTags []string
}

// getTags will get the list of packages in the source directory and then return all the build tags
// in those packages.  One can ignore command packages (packages that are called main) by setting
// ignoreCommands to true, as well as provide a list of packages to ignore. The list of `ignorePackages`
// should be the full path of to the package as defined by `go.mod`
// The returned list of tags will be sorted, and unique. The function, also, ignores the vendor
// directory.
func getTags(sourceDir string, options *getTagsOptions) ([]string, error) {
	if options == nil {
		options = new(getTagsOptions)
	}
	pkgPaths, err := getPackages(sourceDir)
	if err != nil {
		return nil, err
	}

	ignoreTags := make([]*regexp.Regexp, 0, len(options.IgnoreTags))
	// let's compile our regexps
	for _, pattern := range options.IgnoreTags {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile tag pattern `%v`: %w", pattern, err)
		}
		ignoreTags = append(ignoreTags, regex)
	}
	tagsMap := make(map[string]struct{}, 0)
NextPackagePath:
	for _, pkgPath := range pkgPaths {
		for _, iPkgPath := range options.IgnorePackages {
			if pkgPath == iPkgPath {
				continue NextPackagePath
			}
		}
		pkg, err := build.Import(pkgPath, sourceDir, build.IgnoreVendor)
		if err != nil {
			return nil, err
		}
		if options.IgnoreCommands && pkg.IsCommand() {
			continue
		}
		if len(pkg.AllTags) == 0 {
			continue
		}
	NextTag:
		for _, tag := range pkg.AllTags {
			// do we ignore this tag
			for _, pattern := range ignoreTags {
				if pattern.Match([]byte(tag)) {
					continue NextTag
				}
			}
			tagsMap[tag] = struct{}{}
		}
	}
	tags := make([]string, 0, len(tagsMap))
	for tag, _ := range tagsMap {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}

func buildPackageSource(sourceDir, pkgPath string) (name, dir string, err error) {
	pkg, err := build.Import(pkgPath, sourceDir, build.IgnoreVendor)
	if err != nil {
		return "", "", err
	}
	return pkg.Name, pkg.Dir, nil
}

// BuildTemplateSource is the template used to generate the file
// we are using the old style go1.6 and below build tag for now
// until we know for sure that go1.6 is no longer supported
// With go1.7+ the new format is:
// `// go:build ...`
// go format will add this for us.
const BuildTemplateSource = `// +build {{.Tag}}

// This file was autogenerated DO NOT EDIT
// the file was generated with the following command {{.Command}}

package {{.PackageName}}

func init() {
	 // add {{.Tag}} to the Tags
	 {{.TagVar}} = append({{.TagVar}},"{{.Tag}}")
}
`

var BuildTemplate = template.Must(template.New("file").Parse(BuildTemplateSource))

type BuildTag struct {
	Command     string
	PackageName string
	TagVar      string
	Tag         string
}

func writeBuildFileForTag(filename string, tag BuildTag) error {
	// write out the non not version first
	var body bytes.Buffer
	err := BuildTemplate.Execute(&body, tag)
	if err != nil {
		return fmt.Errorf("tag %v errored: %w", tag.Tag, err)
	}
	src, err := format.Source(body.Bytes())
	if err != nil {
		if *verbose {
			fmt.Printf("Tag[%s] %v Body:\n%s\n", tag.Tag, filename, body.Bytes())
		}
		return fmt.Errorf("tag %v format errored: %w", tag.Tag, err)
	}
	return os.WriteFile(filename, src, 0666)
}

// WriteBuildFilesForTag will write out a set of file for the given tag, and it's not version
// the files will be named ${snake_case(tag)}.generated.go and not_${snake_case(tag)}.generated.go
// in the package src dir
func WriteBuildFilesForTag(packageSrcDir string, tag BuildTag) error {
	tagFilename := ToSnakeCase(tag.Tag) + ".generated.go"
	filename := filepath.Join(packageSrcDir, tagFilename)
	err := writeBuildFileForTag(filename, tag)
	if err != nil {
		return err
	}
	if *verbose {
		fmt.Println("wrote file: ", filename)
	}

	tagFilename = "not_" + tagFilename
	filename = filepath.Join(packageSrcDir, tagFilename)
	tag.Tag = "!" + tag.Tag
	err = writeBuildFileForTag(filename, tag)
	if err != nil {
		return err
	}
	if *verbose {
		fmt.Println("wrote file: ", filename)
	}
	return nil
}

// removeGeneratedFiles will remove all files that end with .generated.go
// from the build package directory
func removeGeneratedFiles(packageDir string) error {
	files, err := os.ReadDir(packageDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if *verbose {
			fmt.Printf("checking to see if we can remove %v ", file.Name())
		}
		if strings.HasSuffix(file.Name(), ".generated.go") {
			if *verbose {
				fmt.Printf("yes\n")
			}
			// if we fail to remove it, that's fine
			_ = os.Remove(filepath.Join(packageDir, file.Name()))
		} else {
			if *verbose {
				fmt.Printf(" no\n")
			}
		}
	}
	return nil
}

func main() {

	flag.Parse()

	command := strings.Join(os.Args, " ")
	if *RunCommand != "" {
		command = *RunCommand
	}
	dir := flag.Arg(0)
	if *SrcDir != "" {
		dir = *SrcDir
	}
	if dir == "" {
		dir = "."
	}

	name, src, err := buildPackageSource(dir, *PackageName)
	if err != nil {
		panic(err)
	}

	tags, err := getTags(dir, &getTagsOptions{
		IgnoreCommands: true,
		IgnorePackages: []string{*PackageName},
		IgnoreTags: []string{
			"^ignore$",
			"^cgo$", // we add in "cgo" later
			"^go1.\\d+$",
		},
	})
	if err != nil {
		panic(err)
	}

	tags = append(tags, "cgo", "pprof")

	if !*KeepOldFiles {
		err = removeGeneratedFiles(src)
		if err != nil {
			panic(err)
		}
	}

	for _, tag := range tags {
		err = WriteBuildFilesForTag(src, BuildTag{
			Command:     command,
			PackageName: name,
			TagVar:      *TagVar,
			Tag:         tag,
		})
		if err != nil {
			panic(err)
		}
	}
}
