package wfs

import (
	"fmt"

	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
)

// normalizeTransactionQNames resolves names while their XML namespace scopes
// are still available, before the action decoder extracts inner XML fragments.
func normalizeTransactionQNames(body []byte) ([]byte, error) {
	var root fesElement
	if err := decodeDocument(body, &root); err != nil {
		return nil, err
	}
	rootBindings := namespaceBindings(nil, root.Attrs)
	for i := range root.Children {
		action := &root.Children[i]
		bindings := namespaceBindings(rootBindings, action.Attrs)
		collection := ""
		switch action.XMLName.Local {
		case "Update", "Delete":
			for j := range action.Attrs {
				attr := &action.Attrs[j]
				if attr.Name.Local != "typeName" && attr.Name.Local != "typeNames" {
					continue
				}
				local, err := resolveTypeQName(attr.Value, bindings)
				if err != nil {
					return nil, err
				}
				if collection != "" && collection != local {
					return nil, fmt.Errorf("conflicting transaction type names")
				}
				attr.Value = local
				collection = local
			}
		case "Insert", "Replace":
			for j := range action.Children {
				child := &action.Children[j]
				if child.XMLName.Local == "Filter" {
					continue
				}
				if err := validateFeatureElementNames(child); err != nil {
					return nil, err
				}
				collection = child.XMLName.Local
			}
		}
		for j := range action.Children {
			child := &action.Children[j]
			isFilter := child.XMLName.Local == "Filter" && action.XMLName.Local != "Insert"
			isProperty := child.XMLName.Local == "Property" && action.XMLName.Local == "Update"
			if !isFilter && !isProperty {
				continue
			}
			if err := normalizeActionReferences(child, collection, bindings); err != nil {
				return nil, err
			}
		}
	}
	return marshalNamespaceTree(root)
}

func validateFeatureElementNames(n *fesElement) error {
	collection := n.XMLName.Local
	if !gml.ValidNCName(collection) {
		return fmt.Errorf("invalid feature type name %q", collection)
	}
	if ns := n.XMLName.Space; ns != "" && ns != applicationNamespaceBase+collection {
		return fmt.Errorf("feature namespace does not identify collection %q", collection)
	}
	for _, property := range n.Children {
		if !gml.ValidNCName(property.XMLName.Local) {
			return fmt.Errorf("invalid property name")
		}
		ns := property.XMLName.Space
		if ns == "" || ns == applicationNamespaceBase+collection {
			continue
		}
		isGML := ns == "http://www.opengis.net/gml" || ns == "http://www.opengis.net/gml/3.2"
		if isGML && isGMLGeometryElement(property.XMLName.Local) {
			continue
		}
		return fmt.Errorf("property namespace does not identify collection %q", collection)
	}
	return nil
}

func normalizeActionReferences(n *fesElement, collection string, parent map[string]string) error {
	bindings := namespaceBindings(parent, n.Attrs)
	ns := n.XMLName.Space
	isWFS := ns == "http://www.opengis.net/wfs" || ns == "http://www.opengis.net/wfs/2.0" || ns == ""
	isFilterNS := ns == "http://www.opengis.net/ogc" || ns == "http://www.opengis.net/fes/2.0" || ns == ""
	switch n.XMLName.Local {
	case "Property", "Name", "ValueReference", "Value":
		if !isWFS {
			return fmt.Errorf("unsupported WFS property namespace %q", ns)
		}
	case "Filter", "ResourceId", "FeatureId":
		if !isFilterNS {
			return fmt.Errorf("unsupported feature filter namespace %q", ns)
		}
	}
	switch n.XMLName.Local {
	case "Name", "ValueReference":
		local, err := resolvePropertyQName(n.Text, collection, bindings)
		if err != nil {
			return err
		}
		n.Text = local
	case "Value":
		// Geometry XML and scalar content are not protocol references.
		return nil
	case "ResourceId", "FeatureId":
		for i := range n.Attrs {
			attr := &n.Attrs[i]
			if attr.Name.Local != "rid" && attr.Name.Local != "fid" {
				continue
			}
			id, err := resolveFeatureID(attr.Value, collection, bindings)
			if err != nil {
				return err
			}
			encoded, err := feature.EncodeWFSFID(collection, id)
			if err != nil {
				return err
			}
			attr.Value = encoded
		}
	}
	for i := range n.Children {
		if err := normalizeActionReferences(&n.Children[i], collection, bindings); err != nil {
			return err
		}
	}
	return nil
}
