package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
)

// The feature presentation is independent of the optional tile viewer.
//
//go:embed templates/features.html
var featureHTMLSource string

var featureHTMLTemplate = template.Must(template.New("features").Parse(featureHTMLSource))

// Only handlers supply protocol links. Feature property maps are never inspected
// for hrefs, so a property named links remains ordinary escaped data.
type featureHTMLDocument struct {
	Title, Description string
	Value              any
	Links              []featureLink
}

type featureHTMLView struct {
	Title, Description string
	JSON               string
	Links              []featureLink
}

func renderFeatureHTML(ctx context.Context, document featureHTMLDocument) ([]byte, error) {
	var output bytes.Buffer
	if err := renderFeatureHTMLTo(ctx, &output, document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// renderFeatureHTMLTo writes to the caller's bounded sink and preserves writer
// and cancellation errors. The HTTP writer buffers before committing headers.
func renderFeatureHTMLTo(ctx context.Context, writer io.Writer, document featureHTMLDocument) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if writer == nil {
		return fmt.Errorf("features: nil HTML writer")
	}
	raw, err := json.MarshalIndent(document.Value, "", "  ")
	if err != nil {
		return fmt.Errorf("features: encode HTML response model: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	view := featureHTMLView{Title: document.Title, Description: document.Description, JSON: string(raw), Links: append([]featureLink(nil), document.Links...)}
	if err := featureHTMLTemplate.Execute(featureHTMLContextWriter{ctx: ctx, writer: writer}, view); err != nil {
		return fmt.Errorf("features: render HTML: %w", err)
	}
	return ctx.Err()
}

type featureHTMLContextWriter struct {
	ctx    context.Context
	writer io.Writer
}

func (w featureHTMLContextWriter) Write(value []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := w.writer.Write(value)
	if err == nil && n != len(value) {
		err = io.ErrShortWrite
	}
	return n, err
}
