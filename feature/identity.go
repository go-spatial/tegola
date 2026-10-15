package feature

import (
	"fmt"
	"strconv"
	"strings"
)

// PhysicalFeatureKey identifies one stored object independently of the
// collection name or protocol used to address it. Locks, revisions and
// audit use this key. See ADR-0011.
type PhysicalFeatureKey struct {
	// Domain is the transaction domain (one native writer scope).
	Domain string
	// Relation is the physical relation (table) name.
	Relation string
	// PK is the storage primary key value in canonical form.
	PK string
	// A38: SourceIncarnation identifies the schema generation.
	// Computed at admission from table definition; changes on ALTER.
	SourceIncarnation string
	// A38: EntityIncarnation identifies the entity generation.
	// Incremented on delete/recreate; distinguishes resurrected rows.
	EntityIncarnation uint64
}

func (k PhysicalFeatureKey) String() string {
	base := k.Domain + "." + k.Relation + "." + k.PK
	// A38: append incarnation only when set (backward compatible).
	if k.SourceIncarnation != "" || k.EntityIncarnation != 0 {
		base += "#s" + k.SourceIncarnation + "#e" + strconv.FormatUint(k.EntityIncarnation, 10)
	}
	return base
}

// EncodeWFSFID reversibly encodes collection + numeric ID as a WFS
// feature ID. The result satisfies XML NCName constraints.
// A31: injective NCName-safe encoding via _xHH_ escapes. Unlike the old
// sanitizeNCName (which mapped both "a:b" and "a_b" to "a_b"), this is
// reversible and valid per XML NCName (no '%' which is illegal in NCName).
func EncodeWFSFID(collection string, id uint64) (string, error) {
	if collection == "" {
		return "", fmt.Errorf("empty collection for FID encoding")
	}
	safe := encodeNCName(collection)
	if safe == "" {
		return "", fmt.Errorf("collection %q has no NCName-safe characters", collection)
	}
	return safe + "." + strconv.FormatUint(id, 10), nil
}

// DecodeWFSFID splits a WFS feature ID into collection and numeric ID.
// A31: the collection part is percent-decoded to the original name.
func DecodeWFSFID(fid string) (collection string, id uint64, err error) {
	idx := strings.LastIndex(fid, ".")
	if idx <= 0 || idx == len(fid)-1 {
		return "", 0, fmt.Errorf("malformed feature ID %q", fid)
	}
	enc := fid[:idx]
	collection, err = decodeNCName(enc)
	if err != nil {
		return "", 0, fmt.Errorf("malformed feature ID %q: %v", fid, err)
	}
	n, err := strconv.ParseUint(fid[idx+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("malformed feature ID %q: %v", fid, err)
	}
	return collection, n, nil
}

// encodeNCName percent-encodes chars invalid in XML NCName.
// Valid chars pass through; '%' -> '%25', others -> '%XX'.
// Injective: distinct inputs produce distinct outputs.
// encodeNCName reversibly encodes a string as a valid XML NCName.
// A31: uses _xHH_ escapes (not percent-encoding; '%' is invalid in NCName).
// Valid NCName chars pass through; '_' -> '_x5F_' to avoid ambiguity;
// other invalid chars -> '_xHH_' per UTF-8 byte. Injective and NCName-safe.
func encodeNCName(s string) string {
	var b strings.Builder
	for i, r := range s {
		valid := false
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			valid = true
		case r == '_':
			// Escape underscore to avoid ambiguity with _xHH_ sequences.
			for _, c := range []byte("_") {
				fmt.Fprintf(&b, "_x%02X_", c)
			}
			continue
		case r >= '0' && r <= '9', r == '.', r == '-':
			valid = i != 0
		}
		if valid {
			b.WriteRune(r)
		} else {
			// _xHH_ escape per UTF-8 byte.
			for _, c := range []byte(string(r)) {
				fmt.Fprintf(&b, "_x%02X_", c)
			}
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	// Ensure valid NCName start (letter or underscore).
	if c := out[0]; (c >= '0' && c <= '9') || c == '.' || c == '-' {
		out = "_" + out
	}
	return out
}

// decodeNCName reverses encodeNCName.
func decodeNCName(s string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if i+5 <= len(s) && s[i] == '_' && s[i+1] == 'x' && s[i+4] == '_' {
			// _xHH_ escape.
			var v byte
			_, err := fmt.Sscanf(s[i:i+5], "_x%02X_", &v)
			if err != nil {
				return "", fmt.Errorf("invalid _xHH_ escape in %q", s)
			}
			b.WriteByte(v)
			i += 5
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String(), nil
}

// sanitizeNCName is deprecated; use encodeNCName for injective encoding.
// Kept for backward compatibility with stored data.
func sanitizeNCName(s string) string {
	return encodeNCName(s)
}
