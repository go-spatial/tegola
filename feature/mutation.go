package feature

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/alexeydott/tegola/provider"
)

// MutationCoordinator executes neutral mutations through policy,
// schema validation and a provider transaction. Adapters build
// provider.Mutation values; the coordinator owns the execution order:
// policy → structural validation → Begin → Apply → Commit.
type MutationCoordinator struct {
	// PolicyFor resolves the policy for a collection.
	PolicyFor func(collection string) Policy
	// SchemaFor resolves the schema descriptor for a collection.
	SchemaFor func(collection string) (*SchemaDescriptor, error)
	// SchemaForContext is preferred for request cancellation and deadlines.
	SchemaForContext func(context.Context, string) (*SchemaDescriptor, error)
	// ProviderFor resolves the MutationProvider and provider layer name
	// for a collection.
	ProviderFor func(collection string) (provider.MutationProvider, string, error)
	// OnCommit, if set, is called after successful commit with the
	// mutated collections (A36: for cache invalidation).
	OnCommit func(collections []string)
	// OnInvalidate discards cached reads after a committed or unknown outcome.
	// It is separate from success-only OnCommit notifications.
	OnInvalidate func(collections []string)
	// LockCheck, if set, is called inside the transaction before each
	// Apply (A09: atomic guard). It receives the public collection name
	// and the pinned physical key. Returns non-nil error to abort with
	// MutationErrLockConflict.
	LockCheck func(ctx context.Context, collection string, key PhysicalFeatureKey) error
}

// Execute runs one mutation as its own transaction.
func (c *MutationCoordinator) Execute(ctx context.Context, principal Principal, m provider.Mutation) (provider.MutationOutcome, provider.CommitReceipt, error) {
	outcomes, receipt, err := c.ExecuteAll(ctx, principal, []provider.Mutation{m})
	if len(outcomes) == 0 {
		return provider.MutationOutcome{}, receipt, err
	}
	return outcomes[0], receipt, err
}

// ExecuteAll runs all mutations in one native transaction, in order.
// Later actions see earlier changes. A request spanning two independent
// domains is rejected before any change (ADR-0012).
// sameProvider reports whether two MutationProviders are the same instance.
// R08: pointer equality prevents layer-name confusion across different
// provider instances that share a domain hash.
func sameProvider(a, b provider.MutationProvider) bool {
	// Use reflection-free pointer comparison via interface equality.
	// This works when both are the same concrete pointer type.
	return a == b
}

func (c *MutationCoordinator) ExecuteAll(ctx context.Context, principal Principal, mutations []provider.Mutation) ([]provider.MutationOutcome, provider.CommitReceipt, error) {
	var empty []provider.MutationOutcome
	if len(mutations) == 0 {
		return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
			Kind:   provider.MutationErrMalformedInput,
			Reason: "no mutations",
		}
	}
	mutations = append([]provider.Mutation(nil), mutations...)
	// A02: Policy, provider resolution (pinned), and structural validation
	// happen in a single pass before Begin. The provider instance is pinned
	// at resolution time; the security context (principal) is bound to the
	// pinned provider, preventing TOCTOU between policy check and execution.
	type bound struct {
		mp         provider.MutationProvider
		layer      string
		collection string // original public collection name (for audit)
		policy     Policy
	}
	bounds := make([]bound, len(mutations))
	domain := ""
	for i := range mutations {
		m := &mutations[i]
		origCollection := m.Collection
		action, err := policyActionFor(m.Op.String())
		if err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		policy := c.PolicyFor(origCollection)
		if policy == nil {
			policy = DenyAllPolicy{}
		}
		if d := policy.CheckCollection(ctx, principal, action, origCollection); !d.Allow {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{Kind: provider.MutationErrDenied, Reason: "collection policy denied: " + d.Reason}
		}
		if scope, ok := policy.(CollectionOnlyPolicy); !ok || !scope.CollectionOnly() {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
				Kind:   provider.MutationErrUnsupportedCapability,
				Reason: "row-dependent authorization requires transactional pre-image and post-image support",
			}
		}
		var schema *SchemaDescriptor
		if c.SchemaForContext != nil {
			schema, err = c.SchemaForContext(ctx, origCollection)
		} else {
			schema, err = c.SchemaFor(origCollection)
		}
		if err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
		}
		if err := validateMutationInput(schema, *m); err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
		}
		// A02: pin the provider binding at resolution time.
		mp, layer, err := c.ProviderFor(origCollection)
		if err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
		}
		wd, err := mp.DescribeWritable(ctx, layer)
		if err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
		}
		if domain == "" {
			domain = wd.Domain
		} else if wd.Domain != domain {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
				Kind:   provider.MutationErrDomainMismatch,
				Reason: "transaction spans multiple domains",
			}
		}
		// R08: the tx is opened via bounds[0].mp; all mutations must
		// resolve to the SAME provider instance, not just the same
		// domain hash. Different instances may map the same layer name
		// to different tables/roles.
		if i > 0 && !sameProvider(bounds[0].mp, mp) {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
				Kind:   provider.MutationErrDomainMismatch,
				Reason: "transaction spans multiple provider instances",
			}
		}
		// A02: pin the binding; keep original collection for audit.
		m.Collection = layer
		bounds[i] = bound{mp: mp, layer: layer, collection: origCollection, policy: policy}
	}
	// W13: pass actor for audit. RequestID from context if available.
	requestID, _ := ctx.Value("requestID").(string)
	tx, err := bounds[0].mp.BeginFeatureTx(ctx, provider.TxOptions{
		Actor:     principal.ID,
		RequestID: requestID,
	})
	if err != nil {
		return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
	}
	committed := false
	defer func() {
		if !committed {
			// A37: rollback with bounded cleanup context, not the
			// possibly-cancelled request ctx.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tx.Rollback(cleanupCtx)
		}
	}()
	if err := ctx.Err(); err != nil {
		return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "context cancelled before begin"}
	}
	outcomes := make([]provider.MutationOutcome, 0, len(mutations))
	for i := range mutations {
		// A04: row-level policy check inside the transaction, before Apply.
		// The key uses the bound physical relation (not the public name).
		// A04: post-image check after Apply uses the same key.
		var policy Policy
		var action PolicyAction
		var key PhysicalFeatureKey
		if i < len(bounds) {
			policy = bounds[i].policy
			action, _ = policyActionFor(mutations[i].Op.String())
			key = PhysicalFeatureKey{
				Domain:   domain,
				Relation: bounds[i].layer,
				PK:       formatPK(mutations[i].FeatureID),
			}
			if d := policy.CheckRow(ctx, principal, action, key); !d.Allow {
				return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{Kind: provider.MutationErrDenied, Reason: "row policy denied: " + d.Reason}
			}
		}
		// A09: atomic lock guard inside the transaction, before Apply.
		// The HTTP-layer check is TOCTOU; this re-checks under the tx.
		if c.LockCheck != nil && i < len(bounds) {
			lkey := PhysicalFeatureKey{
				Domain:   domain,
				Relation: bounds[i].layer,
				PK:       formatPK(mutations[i].FeatureID),
			}
			if lerr := c.LockCheck(ctx, bounds[i].collection, lkey); lerr != nil {
				return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
					Kind:   provider.MutationErrLockConflict,
					Reason: lerr.Error(),
				}
			}
		}
		outcome, err := tx.Apply(ctx, mutations[i])
		if err != nil {
			return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, err
		}
		outcomes = append(outcomes, outcome)
		// A04: post-image policy check after Apply, before Commit.
		if i < len(bounds) {
			if d := policy.CheckPostImage(ctx, principal, key, nil); !d.Allow {
				return empty, provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{Kind: provider.MutationErrDenied, Reason: "post-image policy denied: " + d.Reason}
			}
		}
	}
	cols := make([]string, 0, len(mutations))
	seen := map[string]bool{}
	for _, binding := range bounds {
		coll := binding.collection
		if !seen[coll] {
			seen[coll] = true
			cols = append(cols, coll)
		}
	}
	receipt, err := tx.Commit(ctx)
	// A35: preserve the receipt even on commit error, so the caller
	// can distinguish committed/unknown/failed outcomes.
	if receipt.Status == provider.CommitCommitted {
		committed = true
	}
	if c.OnInvalidate != nil && (committed || receipt.Status == provider.CommitUnknown) {
		err = errors.Join(err, notifyMutationCommit(c.OnInvalidate, append([]string(nil), cols...)))
	}
	if !committed {
		if err == nil {
			if receipt.Status == provider.CommitNotCommitted {
				err = &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: "transaction was not committed"}
			} else {
				err = &provider.MutationError{Kind: provider.MutationErrCommitUnknown, Reason: "commit outcome unknown"}
			}
		}
		return empty, receipt, err
	}
	// A43: enrich the durable receipt.
	receipt.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.Actor = principal.ID
	receipt.Collections = cols
	// A36: notify cache invalidation hook.
	if c.OnCommit != nil {
		// An auxiliary notification cannot undo a confirmed database commit.
		// Preserve both a provider auxiliary error and a callback failure.
		err = errors.Join(err, notifyMutationCommit(c.OnCommit, append([]string(nil), cols...)))
	}
	return outcomes, receipt, err
}

// notifyMutationCommit contains callback failures at the post-commit boundary.
func notifyMutationCommit(callback func([]string), collections []string) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("post-commit callback failed: %v", failure)
		}
	}()
	callback(collections)
	return nil
}

// validateMutationInput runs schema validation before Begin: unknown
// properties, read-only properties, type violations, required fields.
func validateMutationInput(schema *SchemaDescriptor, m provider.Mutation) error {
	if m.GeometryUnchanged && (m.Op != provider.MutationReplace || m.IfRevision == "" || m.GeometryWKB != nil || m.GeometryAbsent) {
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "unchanged geometry requires a revision-bound replacement without a geometry payload"}
	}
	if m.Op == provider.MutationDelete {
		if m.FeatureID == 0 {
			return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "delete requires a feature ID"}
		}
		return nil
	}
	missingCompleteGeometry := (m.Op == provider.MutationInsert || m.Op == provider.MutationReplace) && m.GeometryWKB == nil && !m.GeometryUnchanged
	if !schema.Geometry.Nullable && (m.GeometryAbsent || missingCompleteGeometry) {
		return &provider.MutationError{
			Kind:   provider.MutationErrSchemaViolation,
			Reason: "geometry is required and does not accept null",
		}
	}
	if m.Op == provider.MutationInsert && m.FeatureID != 0 {
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "insert must not carry a feature ID"}
	}
	if (m.Op == provider.MutationReplace || m.Op == provider.MutationUpdate) && m.FeatureID == 0 {
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: m.Op.String() + " requires a feature ID"}
	}
	present := make(map[string]TypedValue, len(m.Properties))
	for name, mv := range m.Properties {
		tv := mutationValueToTyped(schema, name, mv)
		present[name] = tv
		if err := schema.ValidateInputValue(name, tv); err != nil {
			return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: err.Error()}
		}
	}
	if m.Op == provider.MutationInsert || m.Op == provider.MutationReplace {
		if err := schema.ValidateRequired(present); err != nil {
			return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: err.Error()}
		}
	}
	return nil
}

func mutationValueToTyped(schema *SchemaDescriptor, name string, mv provider.MutationValue) TypedValue {
	tv := TypedValue{State: ValuePresent}
	if mv.Null {
		tv.State = ValueNull
	}
	if mv.Empty {
		tv.State = ValueEmpty
	}
	if desc, ok := schema.Property(name); ok {
		tv.Type = desc.Type
	}
	switch mv.Kind {
	case provider.MutationValueInteger:
		tv.Integer = mv.Integer
	case provider.MutationValueDecimal:
		tv.Decimal = mv.Decimal
	case provider.MutationValueString:
		tv.String = mv.String
	case provider.MutationValueBoolean:
		tv.Boolean = mv.Boolean
	}
	return tv
}

// formatPK renders a feature ID for policy keys.
func formatPK(id uint64) string {
	return strconv.FormatUint(id, 10)
}
