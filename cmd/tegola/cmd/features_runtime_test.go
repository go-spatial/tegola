package cmd

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/server"
	"github.com/go-spatial/cobra"
)

func TestFeatureRuntimeAssemblyResetAndIsolation(t *testing.T) {
	firstAPI := &FeatureRuntime{API: &server.FeatureAPI{}, WFS: &server.WFSHandler{}}
	secondAPI := &FeatureRuntime{API: &server.FeatureAPI{}, WFS: &server.WFSHandler{}}
	sentinel := errors.New("initialization failed")
	firstRoot, firstServe := &cobra.Command{}, &cobra.Command{}
	secondRoot, secondServe := &cobra.Command{}, &cobra.Command{}
	var firstSeen, secondSeen *FeatureRuntime
	nextAPI, nextError := firstAPI, error(nil)
	bindFeatureRuntime(firstRoot, firstServe, func(*cobra.Command, []string) (*FeatureRuntime, error) {
		return nextAPI, nextError
	}, func(_ *cobra.Command, _ []string, api *FeatureRuntime) error {
		firstSeen = api
		return nil
	})
	bindFeatureRuntime(secondRoot, secondServe, func(*cobra.Command, []string) (*FeatureRuntime, error) {
		return secondAPI, nil
	}, func(_ *cobra.Command, _ []string, api *FeatureRuntime) error {
		secondSeen = api
		return nil
	})
	if err := firstRoot.PersistentPreRunE(firstServe, nil); err != nil {
		t.Fatal(err)
	}
	if err := firstServe.RunE(firstServe, nil); err != nil {
		t.Fatal(err)
	}
	if firstSeen != firstAPI {
		t.Fatal("runtime not carried")
	}
	if err := secondRoot.PersistentPreRunE(secondServe, nil); err != nil {
		t.Fatal(err)
	}
	if err := secondServe.RunE(secondServe, nil); err != nil {
		t.Fatal(err)
	}
	if secondSeen != secondAPI {
		t.Fatal("second assembly affected")
	}
	// A failing initializer returning a partial runtime must not retain either
	// that pointer or the previous successful runtime.
	nextAPI, nextError = secondAPI, sentinel
	if err := firstRoot.PersistentPreRunE(firstServe, nil); !errors.Is(err, sentinel) {
		t.Fatalf("lost error %v", err)
	}
	if err := firstServe.RunE(firstServe, nil); err != nil {
		t.Fatal(err)
	}
	if firstSeen != nil {
		t.Fatal("failed initialization retained runtime")
	}
	nextAPI, nextError = nil, nil
	if err := firstRoot.PersistentPreRunE(firstServe, nil); err != nil {
		t.Fatal(err)
	}
	if err := firstServe.RunE(firstServe, nil); err != nil {
		t.Fatal(err)
	}
	if firstSeen != nil {
		t.Fatal("disabled initialization retained runtime")
	}
	if err := secondServe.RunE(secondServe, nil); err != nil {
		t.Fatal(err)
	}
	if secondSeen != secondAPI {
		t.Fatal("reset leaked across assemblies")
	}
	if firstServe.Run != nil {
		t.Fatal("old Run bypass remains")
	}
}

func TestFeatureRuntimeInitializationPublicationFailure(t *testing.T) {
	previousConfig, previousLogger := conf, slog.Default()
	t.Cleanup(func() { conf = previousConfig; slog.SetDefault(previousLogger) })
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `[features]
enabled = true
[[features.collections]]
id = "roads"
provider_layer = "missing.source"
`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	rt, err := initConfigRuntime(path, false, "ERROR")
	if rt != nil || err == nil || !strings.Contains(err.Error(), "could not register features") {
		t.Fatalf("enabled missing source silently omitted: %v %v", rt, err)
	}
	// Disabled syntactically valid publication does not resolve a missing source.
	body = strings.Replace(body, "enabled = true", "enabled = false", 1)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	rt, err = initConfigRuntime(path, true, "ERROR")
	if rt != nil || err == nil || !strings.Contains(err.Error(), "no cache defined") {
		t.Fatalf("disabled source was resolved: %v %v", rt, err)
	}
}

func TestStockRuntimeRejectsProductionWriteWithoutAuthenticator(t *testing.T) {
	previousConfig, previousLogger := conf, slog.Default()
	t.Cleanup(func() { conf = previousConfig; slog.SetDefault(previousLogger) })
	for _, mode := range []string{"production", ""} {
		t.Run("mode="+mode, func(t *testing.T) {
			body := `[features]
enabled = true
[[features.collections]]
id = "sites"
provider_layer = "missing.sites"
[features.write]
enabled = true
auth_mode = "` + mode + `"
[[features.write.collections]]
id = "sites"
operations = ["create"]
`
			path := filepath.Join(t.TempDir(), "production.toml")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			rt, err := initConfigRuntime(path, false, "ERROR")
			if rt != nil || err == nil || !strings.Contains(err.Error(), "stock executable has no production authenticator") {
				t.Fatalf("expected explicit auth startup rejection: runtime=%v error=%v", rt, err)
			}
		})
	}
}
