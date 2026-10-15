// Command check-dependencies verifies published fork dependencies and an
// independent downstream consumer. -revision also downloads Tegola remotely.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

const tegolaModule = "github.com/alexeydott/tegola"

var forkModules = []string{"github.com/alexeydott/geom", "github.com/alexeydott/proj"}
var releaseVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-fork\.[0-9]+)?$`)

type moduleVersion struct {
	Path    string
	Version string
}

type replacement struct {
	Old moduleVersion
	New moduleVersion
}

type moduleFile struct {
	Module  moduleVersion
	Go      string
	Require []moduleVersion
	Replace []replacement
}

type selectedModule struct {
	Path    string
	Version string
	Dir     string
	Replace *selectedModule
}

type runner struct {
	moduleCache string
}

func main() {
	revision := flag.String("revision", "", "published Tegola revision or tag; downloads all dependencies into a fresh module cache with no replacements")
	flag.Parse()
	if err := check(*revision); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (r runner) goCommand(dir string, offline bool, args ...string) ([]byte, error) {
	goName := "go"
	if runtime.GOOS == "windows" {
		goName += ".exe"
	}
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", goName), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local")
	if r.moduleCache != "" {
		cmd.Env = append(cmd.Env, "GOMODCACHE="+r.moduleCache, "GOFLAGS=-modcacherw",
			"GOPROXY=https://proxy.golang.org,direct", "GOSUMDB=sum.golang.org",
			"GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=")
	}
	if offline {
		cmd.Env = append(cmd.Env, "GOPROXY=off", "GOSUMDB=off")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go %v in %s: %w\n%s", args, dir, err, output)
	}
	return output, nil
}

func (r runner) readModule(dir string) (moduleFile, error) {
	var mod moduleFile
	output, err := r.goCommand(dir, false, "mod", "edit", "-json")
	if err != nil {
		return mod, err
	}
	if err := json.Unmarshal(output, &mod); err != nil {
		return mod, fmt.Errorf("read module in %s: %w", dir, err)
	}
	return mod, nil
}

func forkVersions(mod moduleFile) (map[string]string, error) {
	if len(mod.Replace) != 0 {
		return nil, fmt.Errorf("Tegola must resolve its dependencies without replace directives")
	}
	versions := make(map[string]string, len(forkModules))
	for _, requirement := range mod.Require {
		if requirement.Path == "github.com/go-spatial/geom" || requirement.Path == "github.com/go-spatial/proj" {
			return nil, fmt.Errorf("unpatched upstream dependency %s", requirement.Path)
		}
		for _, path := range forkModules {
			if requirement.Path == path {
				if !releaseVersion.MatchString(requirement.Version) {
					return nil, fmt.Errorf("%s must pin a release tag, got %q", path, requirement.Version)
				}
				versions[path] = requirement.Version
			}
		}
	}
	if len(versions) != len(forkModules) {
		return nil, fmt.Errorf("Tegola must directly require both published fork modules")
	}
	return versions, nil
}

func (r runner) verifyGraph(consumer, root string, remote bool, versions map[string]string) error {
	output, err := r.goCommand(consumer, false, "list", "-mod=readonly", "-m", "-json", "all")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	found := make(map[string]bool)
	for {
		var mod selectedModule
		if err := decoder.Decode(&mod); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if mod.Path == "github.com/go-spatial/geom" || mod.Path == "github.com/go-spatial/proj" {
			return fmt.Errorf("consumer selected unpatched upstream dependency %s", mod.Path)
		}
		if mod.Replace != nil {
			if remote || mod.Path != tegolaModule || filepath.Clean(mod.Replace.Dir) != filepath.Clean(root) {
				return fmt.Errorf("unexpected consumer replacement for %s", mod.Path)
			}
		}
		want, fork := versions[mod.Path]
		if !fork && mod.Path != tegolaModule {
			continue
		}
		if fork && (mod.Version != want || mod.Replace != nil) {
			return fmt.Errorf("consumer selected %s@%s; want published %s", mod.Path, mod.Version, want)
		}
		if mod.Dir == "" {
			return fmt.Errorf("module %s was not downloaded", mod.Path)
		}
		manifest, err := r.readModule(mod.Dir)
		if err != nil {
			return err
		}
		if manifest.Module.Path != mod.Path || len(manifest.Replace) != 0 {
			return fmt.Errorf("module %s has an unexpected module path or replacements", mod.Path)
		}
		found[mod.Path] = true
		fmt.Printf("Consumer selected %s@%s\n", mod.Path, mod.Version)
	}
	if len(found) != len(versions)+1 {
		return fmt.Errorf("consumer graph is missing Tegola or a published fork")
	}
	return nil
}

func check(revision string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	r := runner{}
	mod, err := r.readModule(root)
	if err != nil {
		return err
	}
	if mod.Module.Path != tegolaModule {
		return fmt.Errorf("run this check from the alexeydott/tegola repository root")
	}
	versions, err := forkVersions(mod)
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", "tegola-consumer-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(workspace); err != nil {
			fmt.Fprintf(os.Stderr, "remove temporary consumer %s: %v\n", workspace, err)
		}
	}()
	consumer := filepath.Join(workspace, "consumer")
	if err := os.Mkdir(consumer, 0700); err != nil {
		return err
	}
	if revision != "" {
		r.moduleCache = filepath.Join(workspace, "module-cache")
	}
	goMod := fmt.Sprintf("module example.com/tegola-consumer\n\ngo %s\n", mod.Go)
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(goMod), 0600); err != nil {
		return err
	}
	if revision == "" {
		_, err = r.goCommand(consumer, false, "mod", "edit", "-require="+tegolaModule+"@v0.0.0", "-replace="+tegolaModule+"="+filepath.ToSlash(root))
	} else {
		_, err = r.goCommand(consumer, false, "get", tegolaModule+"@"+revision)
	}
	if err != nil {
		return err
	}
	for _, name := range []string{"consumer.go", "consumer_test.go"} {
		source, err := os.ReadFile(filepath.Join(root, "ci", "check-dependencies", "testdata", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(consumer, name), source, 0600); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"mod", "tidy"},
		{"test", "-mod=readonly", "-count=1", "./..."},
	} {
		output, err := r.goCommand(consumer, false, args...)
		if err != nil {
			return err
		}
		fmt.Printf("Consumer go %v: passed\n%s", args, output)
	}
	if err := r.verifyGraph(consumer, root, revision != "", versions); err != nil {
		return err
	}
	for _, args := range [][]string{{"mod", "verify"}, {"mod", "vendor"}} {
		output, err := r.goCommand(consumer, false, args...)
		if err != nil {
			return err
		}
		fmt.Printf("Consumer go %v: passed\n%s", args, output)
	}
	for _, args := range [][]string{
		{"build", "-mod=vendor", "./..."},
		{"test", "-mod=vendor", "-count=1", "./..."},
	} {
		output, err := r.goCommand(consumer, true, args...)
		if err != nil {
			return err
		}
		fmt.Printf("Offline consumer go %v: passed\n%s", args, output)
	}
	return nil
}
