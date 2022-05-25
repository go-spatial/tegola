//go:build noViewer
// +build noViewer

package build

func ViewerVersion() string {
	return uiVersionDefaultText
}
