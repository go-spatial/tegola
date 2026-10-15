// Package wfs implements the WFS 1.1.0 / 2.0 transport adapters.
// Adapters parse KVP/XML into neutral commands and render
// version-specific responses; business logic lives in feature/ and
// provider/. See ADR-0010.
package wfs

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Version is a WFS wire version.
type Version string

const (
	V110 Version = "1.1.0"
	V200 Version = "2.0.0"
	V202 Version = "2.0.2"
)

// SupportedVersions lists the advertised versions.
var SupportedVersions = []Version{V110, V200, V202}

// Negotiate selects the wire version: explicit version, highest
// supported, or error for unsupported.
func Negotiate(requested string, accepted []string) (Version, error) {
	if requested != "" {
		for _, v := range SupportedVersions {
			if string(v) == requested {
				return v, nil
			}
		}
		return "", fmt.Errorf("unsupported WFS version %q", requested)
	}
	if len(accepted) > 0 {
		for i := len(SupportedVersions) - 1; i >= 0; i-- {
			v := SupportedVersions[i]
			for _, a := range accepted {
				if string(v) == strings.TrimSpace(a) {
					return v, nil
				}
			}
		}
		return "", fmt.Errorf("no supported WFS version in acceptversions")
	}
	return V202, nil
}

// ExceptionCode is an OWS exception code.
type ExceptionCode string

const (
	ExceptionInvalidParameterValue ExceptionCode = "InvalidParameterValue"
	ExceptionMissingParameterValue ExceptionCode = "MissingParameterValue"
	ExceptionOperationNotSupported ExceptionCode = "OperationNotSupported"
	ExceptionNoApplicableCode      ExceptionCode = "NoApplicableCode"
)

// Exception is one OWS exception.
type Exception struct {
	Code    ExceptionCode
	Locator string
	Text    string
}

// ExceptionReport renders an OWS ExceptionReport for the version.
func ExceptionReport(v Version, errs []Exception) string {
	var sb strings.Builder
	if v == V110 {
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		sb.WriteString(`<ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ows http://schemas.opengis.net/ows/1.0.0/owsExceptionReport.xsd" version="1.0.0" xml:lang="en">` + "\n")
	} else {
		sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		sb.WriteString(`<ows:ExceptionReport xmlns:ows="http://www.opengis.net/ows/1.1" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.opengis.net/ows/1.1 http://schemas.opengis.net/ows/1.1.0/owsExceptionReport.xsd" version="2.0.0" xml:lang="en">` + "\n")
	}
	for _, e := range errs {
		sb.WriteString(`  <ows:Exception exceptionCode="` + string(e.Code) + `"`)
		if e.Locator != "" {
			sb.WriteString(` locator="` + xmlEscape(e.Locator) + `"`)
		}
		sb.WriteString(">\n")
		sb.WriteString(`    <ows:ExceptionText>` + xmlEscape(e.Text) + "</ows:ExceptionText>\n")
		sb.WriteString("  </ows:Exception>\n")
	}
	sb.WriteString("</ows:ExceptionReport>\n")
	return sb.String()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// decodeDocument rejects trailing XML roots instead of silently executing only
// the first request in a malformed document.
func decodeDocument(body []byte, out interface{}) error {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	if err := dec.Decode(out); err != nil {
		return err
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return fmt.Errorf("unexpected trailing text")
			}
		case xml.Comment:
		default:
			return fmt.Errorf("unexpected trailing XML content")
		}
	}
}

// ParseVersionedRequest detects the WFS version from an XML document
// root element namespace.
func ParseVersionedRequest(body []byte) (Version, string, error) {
	type root struct {
		XMLName xml.Name
		Version string `xml:"version,attr"`
	}
	var r root
	if err := decodeDocument(body, &r); err != nil {
		return "", "", fmt.Errorf("invalid XML: %w", err)
	}
	local := r.XMLName.Local
	switch r.XMLName.Space {
	case "http://www.opengis.net/wfs":
		// WFS 1.1 namespace.
		if r.Version == "" || r.Version == "1.1.0" {
			return V110, local, nil
		}
		return "", "", fmt.Errorf("unsupported WFS 1.x version %q", r.Version)
	case "http://www.opengis.net/wfs/2.0":
		if r.Version == "2.0.0" {
			return V200, local, nil
		}
		if r.Version == "" || r.Version == "2.0.0" || r.Version == "2.0.2" {
			return V202, local, nil
		}
		return "", "", fmt.Errorf("unsupported WFS 2.x version %q", r.Version)
	default:
		return "", "", fmt.Errorf("not a WFS request (namespace %q)", r.XMLName.Space)
	}
}
