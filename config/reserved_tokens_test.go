package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// paramTOML renders a minimal config with a single map declaring one custom
// query parameter with the given token.
func paramTOML(token string) string {
	return "[[maps]]\nname = \"test\"\n" +
		"  [[maps.params]]\n" +
		"  name = \"pa\"\n" +
		"  token = \"" + token + "\"\n" +
		"  type = \"string\"\n"
}

// loadAndValidateTOML writes the given config to a temp file and runs the
// full load+validate reload path against it.
func loadAndValidateTOML(t *testing.T, tomlSrc string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(tomlSrc), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAndValidate(path)
	return err
}

// TestReservedTokensReloadSameToken ensures two consecutive loads of a config
// declaring the same custom parameter token do not share token state: the
// second load must not reject the token as reserved (part13 P6-28).
func TestReservedTokensReloadSameToken(t *testing.T) {
	src := paramTOML("!TOKA!")

	if err := loadAndValidateTOML(t, src); err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if err := loadAndValidateTOML(t, src); err != nil {
		t.Fatalf("second load failed (token state leaked across loads?): %v", err)
	}
}

// TestReservedTokensNotAccumulated ensures reserved tokens registered by a
// previous load do not survive into the next one: the provider-facing mirror
// must be replaced wholesale, never merged (part13 P6-28).
func TestReservedTokensNotAccumulated(t *testing.T) {
	if err := loadAndValidateTOML(t, paramTOML("!TOKC!")); err != nil {
		t.Fatalf("first load failed: %v", err)
	}
	if err := loadAndValidateTOML(t, paramTOML("!TOKD!")); err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if _, ok := ReservedTokens["!TOKD!"]; !ok {
		t.Error("expected !TOKD! (latest config's token) in the ReservedTokens mirror")
	}
	if _, ok := ReservedTokens["!TOKC!"]; ok {
		t.Error("stale !TOKC! from the previous load leaked into the ReservedTokens mirror")
	}
}

// TestReservedTokensBuiltinsStillReserved ensures the per-config refactor does
// not open up the built-in tokens for use as custom parameter tokens
// (part13 P6-28).
func TestReservedTokensBuiltinsStillReserved(t *testing.T) {
	err := loadAndValidateTOML(t, paramTOML(BboxToken))
	if err == nil {
		t.Fatal("expected a config param reusing the builtin token !BBOX! to be rejected")
	}
	var reserved ErrParamTokenReserved
	if !errors.As(err, &reserved) {
		t.Errorf("expected ErrParamTokenReserved, got: %v", err)
	}
}

// TestReservedTokensMirrorHasBuiltins ensures the provider-facing mirror keeps
// the built-in tokens after a validation so providers (e.g. postgis) still
// recognize them (part13 P6-28).
func TestReservedTokensMirrorHasBuiltins(t *testing.T) {
	if err := loadAndValidateTOML(t, "[[maps]]\nname = \"test\"\n"); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	for _, tok := range []string{BboxToken, ZoomToken, XToken, YToken, ZToken, ScaleDenominatorToken, PixelWidthToken, PixelHeightToken, IdFieldToken, GeomFieldToken, GeomTypeToken} {
		if _, ok := ReservedTokens[tok]; !ok {
			t.Errorf("expected builtin token %s in the ReservedTokens mirror", tok)
		}
	}
}
