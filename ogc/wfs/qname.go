package wfs

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
)

const applicationNamespaceBase = "http://example.com/tegola/"

func splitQName(value string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) == 1 && gml.ValidNCName(parts[0]) {
		return "", parts[0], nil
	}
	if len(parts) == 2 && gml.ValidNCName(parts[0]) && gml.ValidNCName(parts[1]) {
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("invalid QName %q", value)
}

// kvpNamespaces supports the WFS 2 comma and WFS 1.1 equals binding syntax.
func kvpNamespaces(params map[string]string) (map[string]string, error) {
	raw := params["namespaces"]
	if other := params["namespace"]; other != "" {
		if raw != "" && raw != other {
			return nil, fmt.Errorf("conflicting namespace parameters")
		}
		raw = other
	}
	bindings := map[string]string{}
	for strings.TrimSpace(raw) != "" {
		raw = strings.TrimSpace(raw)
		if !strings.HasPrefix(raw, "xmlns(") {
			return nil, fmt.Errorf("invalid namespace binding")
		}
		end := strings.IndexByte(raw, ')')
		if end < 0 {
			return nil, fmt.Errorf("unclosed namespace binding")
		}
		content := raw[len("xmlns("):end]
		i := strings.IndexAny(content, ",=")
		if i < 1 {
			return nil, fmt.Errorf("namespace binding requires prefix and URI")
		}
		prefix, uri := strings.TrimSpace(content[:i]), strings.TrimSpace(content[i+1:])
		if !gml.ValidNCName(prefix) || uri == "" || strings.ContainsAny(uri, "(),") {
			return nil, fmt.Errorf("invalid namespace binding")
		}
		if _, exists := bindings[prefix]; exists {
			return nil, fmt.Errorf("duplicate namespace prefix %q", prefix)
		}
		bindings[prefix] = uri
		raw = strings.TrimSpace(raw[end+1:])
		if raw != "" {
			if raw[0] != ',' {
				return nil, fmt.Errorf("invalid namespace separator")
			}
			raw = raw[1:]
			if strings.TrimSpace(raw) == "" {
				return nil, fmt.Errorf("missing namespace binding")
			}
		}
	}
	return bindings, nil
}

func resolveTypeQName(value string, bindings map[string]string) (string, error) {
	prefix, local, err := splitQName(value)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return local, nil
	}
	uri, exists := bindings[prefix]
	if !exists {
		return "", fmt.Errorf("unknown namespace prefix %q", prefix)
	}
	if uri != applicationNamespaceBase+local {
		return "", fmt.Errorf("namespace does not identify collection %q", local)
	}
	return local, nil
}

// ResolveTypeNameKVP resolves the advertised app QName or explicit namespace bindings.
func ResolveTypeNameKVP(value string, params map[string]string) (string, error) {
	bindings, err := kvpNamespaces(params)
	if err != nil {
		return "", err
	}
	prefix, local, err := splitQName(value)
	if err != nil {
		return "", err
	}
	if prefix == "app" {
		if _, bound := bindings[prefix]; !bound {
			bindings[prefix] = applicationNamespaceBase + local
		}
	}
	return resolveTypeQName(value, bindings)
}

func propertyBindingsKVP(collection string, params map[string]string) (map[string]string, error) {
	bindings, err := kvpNamespaces(params)
	if err != nil {
		return nil, err
	}
	if _, bound := bindings["app"]; !bound {
		bindings["app"] = applicationNamespaceBase + collection
	}
	return bindings, nil
}

func resolvePropertyQName(value, collection string, bindings map[string]string) (string, error) {
	prefix, local, err := splitQName(value)
	if err != nil {
		return "", err
	}
	if prefix == "" {
		return local, nil
	}
	uri, exists := bindings[prefix]
	if !exists {
		return "", fmt.Errorf("unknown namespace prefix %q", prefix)
	}
	if collection != "" {
		if uri != applicationNamespaceBase+collection {
			return "", fmt.Errorf("property namespace does not identify collection %q", collection)
		}
	} else if !strings.HasPrefix(uri, applicationNamespaceBase) || !gml.ValidNCName(strings.TrimPrefix(uri, applicationNamespaceBase)) {
		return "", fmt.Errorf("unsupported application namespace %q", uri)
	}
	return local, nil
}

func resolveFeatureID(value, collection string, bindings map[string]string) (uint64, error) {
	idx := strings.LastIndex(value, ".")
	if idx > 0 && strings.Contains(value[:idx], ":") {
		local, err := resolveTypeQName(value[:idx], bindings)
		if err != nil {
			return 0, err
		}
		encoded, err := feature.EncodeWFSFID(local, 0)
		if err != nil {
			return 0, err
		}
		value = strings.TrimSuffix(encoded, "0") + value[idx+1:]
	}
	actual, id, err := feature.DecodeWFSFID(value)
	if err != nil {
		return 0, err
	}
	if actual != collection {
		return 0, fmt.Errorf("feature ID does not identify collection %q", collection)
	}
	return id, nil
}

func namespaceBindings(parent map[string]string, attrs []xml.Attr) map[string]string {
	out := make(map[string]string, len(parent)+len(attrs))
	for prefix, uri := range parent {
		out[prefix] = uri
	}
	for _, a := range attrs {
		if a.Name.Space == "xmlns" {
			out[a.Name.Local] = a.Value
		}
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			out[""] = a.Value
		}
	}
	return out
}

// marshalNamespaceTree re-emits resolved element/attribute names. QName-valued
// text and attributes must be resolved before dropping the original bindings.
func marshalNamespaceTree(root fesElement) ([]byte, error) {
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	var write func(fesElement) error
	write = func(n fesElement) error {
		attrs := make([]xml.Attr, 0, len(n.Attrs))
		for _, a := range n.Attrs {
			if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" {
				attrs = append(attrs, a)
			}
		}
		start := xml.StartElement{Name: n.XMLName, Attr: attrs}
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.CharData(n.Text)); err != nil {
			return err
		}
		for _, child := range n.Children {
			if err := write(child); err != nil {
				return err
			}
		}
		return enc.EncodeToken(start.End())
	}
	if err := write(root); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func normalizeFESQNames(body []byte, collection string, initial map[string]string) ([]byte, error) {
	var root fesElement
	if err := decodeDocument(body, &root); err != nil {
		return nil, err
	}
	var walk func(*fesElement, map[string]string) error
	walk = func(n *fesElement, parent map[string]string) error {
		bindings := namespaceBindings(parent, n.Attrs)
		if n.XMLName.Local == "ValueReference" || n.XMLName.Local == "PropertyName" {
			name, err := resolvePropertyQName(n.Text, collection, bindings)
			if err != nil {
				return err
			}
			n.Text = name
		}
		for i := range n.Children {
			if err := walk(&n.Children[i], bindings); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(&root, initial); err != nil {
		return nil, err
	}
	return marshalNamespaceTree(root)
}
