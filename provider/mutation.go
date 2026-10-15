package provider

import (
	"context"
	"errors"
	"fmt"
)

// MutationOp is the neutral write operation. Adapters (WFS Transaction
// actions, Part 4 HTTP methods) map to these; SQL is only reachable from
// provider implementations. See ADR-0010.
type MutationOp int

const (
	MutationInsert MutationOp = iota
	MutationReplace
	MutationUpdate
	MutationDelete
)

func (op MutationOp) String() string {
	switch op {
	case MutationInsert:
		return "insert"
	case MutationReplace:
		return "replace"
	case MutationUpdate:
		return "update"
	case MutationDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// Mutation is one neutral write command. Properties maps public property
// names to values; geometry arrives as WKB in the storage CRS unless
// GeometrySRID says otherwise. For Insert, FeatureID must be zero;
// for Replace/Update/Delete it identifies the target.
type Mutation struct {
	Op         MutationOp
	Collection string
	FeatureID  uint64
	// Properties holds public property inputs (absent/null/empty/value
	// states preserved). Geometry is separate.
	Properties map[string]MutationValue
	// GeometryWKB is the input geometry; nil means "not provided".
	// GeometryAbsent, when true, explicitly clears the geometry where
	// the schema allows it.
	GeometryWKB    []byte
	GeometryAbsent bool
	// GeometrySRID is the CRS of GeometryWKB (0 = storage CRS).
	GeometrySRID uint64
	// GeometryUnchanged marks a complete replacement whose supplied geometry
	// exactly matches the canonical read representation at IfRevision. It keeps
	// native geometry bytes/bounds without another lossy projection round trip.
	// Only Replace with a non-empty revision and absent geometry payload may use it.
	GeometryUnchanged bool
	// IfRevision, when non-empty, requires the target's current revision
	// to equal it (optimistic concurrency). Empty means no precondition.
	IfRevision string
}

// MutationValue is the neutral input value. Integer is int64, Decimal is
// the canonical decimal string; neither passes through float64.
type MutationValue struct {
	// Null, when true, is an explicit null (distinct from absent key).
	Null bool
	// Empty, when true, is an explicit empty value.
	Empty   bool
	Integer int64
	Decimal string
	String  string
	Boolean bool
	// Kind identifies which field carries the value.
	Kind MutationValueKind
}

// MutationValueKind selects the active MutationValue field.
type MutationValueKind int

const (
	MutationValueUnset MutationValueKind = iota
	MutationValueInteger
	MutationValueDecimal
	MutationValueString
	MutationValueBoolean
)

// MutationOutcome is the per-command result of Apply.
type MutationOutcome struct {
	FeatureID uint64
	// Revision is the post-write revision token.
	Revision string
	// RevisionBefore is the pre-write revision token (R12).
	RevisionBefore string
	// Affected is the number of stored rows affected (0 or 1 for the
	// single-object profile).
	Affected int
}

// CommitStatus distinguishes the three commit outcomes. A lost
// acknowledgement after COMMIT is Unknown, never silently retried.
type CommitStatus int

const (
	CommitUnknown CommitStatus = iota
	CommitCommitted
	CommitNotCommitted
)

// CommitReceipt is returned by FeatureTx.Commit.
type CommitReceipt struct {
	Status CommitStatus
	// TransactionID is the provider's native transaction identifier,
	// for audit correlation.
	TransactionID string
	// A43: durable receipt fields.
	// Timestamp is the commit time (UTC).
	Timestamp string
	// Actor is the principal that performed the mutation.
	Actor string
	// Collections lists the mutated collections.
	Collections []string
}

// TxOptions configures BeginFeatureTx.
type TxOptions struct {
	// ReadOnly, when true, begins a read-only transaction (for
	// transaction-bound selection without writes).
	ReadOnly bool
	// Actor identifies the principal performing the mutation (for audit).
	Actor string
	// RequestID correlates the transaction with the HTTP request (for audit).
	RequestID string
}

// FeatureTx is one native transaction. Apply executes transaction-bound
// selection, locking, revision/authorization checks and post-image
// validation itself; implementations must not call the read-snapshot
// FeatureQuerier for state-dependent checks.
type FeatureTx interface {
	Apply(ctx context.Context, command Mutation) (MutationOutcome, error)
	Commit(ctx context.Context) (CommitReceipt, error)
	Rollback(ctx context.Context) error
}

// MutationProvider is implemented by backends that pass write admission.
type MutationProvider interface {
	// DescribeWritable returns the write descriptor for a layer, or an
	// error when the layer is not admitted for writing.
	DescribeWritable(ctx context.Context, layer string) (WriteDescriptor, error)
	// BeginFeatureTx starts a native transaction covering one domain.
	BeginFeatureTx(ctx context.Context, options TxOptions) (FeatureTx, error)
}

// RevisionReader is implemented by providers that track per-feature
// revisions (A03). CurrentRevision returns "0" when no revision exists.
type RevisionReader interface {
	CurrentRevision(ctx context.Context, layer string, featureID uint64) (string, error)
}

// MutationErrorKind is the taxonomy from ADR-0012. Adapters map each
// kind to their protocol-specific response; not every kind is a 409.
type MutationErrorKind int

const (
	MutationErrMalformedInput MutationErrorKind = iota
	MutationErrSchemaViolation
	MutationErrDenied
	MutationErrNotFound
	MutationErrPreconditionFailed
	MutationErrLockConflict
	MutationErrUnsupportedCapability
	MutationErrQuotaExceeded
	MutationErrDomainMismatch
	MutationErrCommitUnknown
	MutationErrExecutionFailed
)

// MutationError carries a classified mutation failure. Reason must not
// leak DSN/SQL/schema internals.
type MutationError struct {
	Kind   MutationErrorKind
	Reason string
}

func (e *MutationError) Error() string {
	return fmt.Sprintf("mutation %s: %s", e.Kind, e.Reason)
}

func (k MutationErrorKind) String() string {
	switch k {
	case MutationErrMalformedInput:
		return "malformed input"
	case MutationErrSchemaViolation:
		return "schema violation"
	case MutationErrDenied:
		return "denied"
	case MutationErrNotFound:
		return "not found"
	case MutationErrPreconditionFailed:
		return "precondition failed"
	case MutationErrLockConflict:
		return "lock conflict"
	case MutationErrUnsupportedCapability:
		return "unsupported capability"
	case MutationErrQuotaExceeded:
		return "quota exceeded"
	case MutationErrDomainMismatch:
		return "transaction-domain mismatch"
	case MutationErrExecutionFailed:
		return "execution failed"
	case MutationErrCommitUnknown:
		return "commit unknown"
	default:
		return "unknown"
	}
}

// AsMutationError extracts the *MutationError from err, if present.
func AsMutationError(err error) (*MutationError, bool) {
	var me *MutationError
	if errors.As(err, &me) {
		return me, true
	}
	return nil, false
}
