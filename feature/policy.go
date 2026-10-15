package feature

import (
	"context"
	"fmt"
)

// Principal is the authenticated actor. Identity comes from a verified
// auth adapter (validated token or trusted gateway), never from an
// arbitrary X-User header.
type Principal struct {
	// ID is the stable actor identifier.
	ID string
	// Anonymous, when true, carries no authenticated identity.
	Anonymous bool
}

// PolicyAction distinguishes the checked operations. Read, insert,
// replace, update, delete and lock are separate permissions.
type PolicyAction int

const (
	ActionRead PolicyAction = iota
	ActionInsert
	ActionReplace
	ActionUpdate
	ActionDelete
	ActionLock
)

// PolicyDecision is allow or deny with a reason.
type PolicyDecision struct {
	Allow  bool
	Reason string
}

// Policy checks collection, property set and row-level rights, including
// the post-image of update/replace (a payload must not move a row out of
// its allowed tenant/owner scope). Deny-by-default.
type Policy interface {
	// CheckCollection authorizes an action on a whole collection.
	CheckCollection(ctx context.Context, p Principal, action PolicyAction, collection string) PolicyDecision
	// CheckRow authorizes an action on one physical row.
	CheckRow(ctx context.Context, p Principal, action PolicyAction, key PhysicalFeatureKey) PolicyDecision
	// CheckPostImage authorizes the resulting state of an update/replace.
	// It receives the pre-image key and the proposed properties.
	CheckPostImage(ctx context.Context, p Principal, key PhysicalFeatureKey, proposed map[string]TypedValue) PolicyDecision
}

// DenyAllPolicy denies every mutation and allows reads. It is the
// default when no policy is configured: write stays disabled.
type DenyAllPolicy struct{}

func (DenyAllPolicy) CheckCollection(_ context.Context, _ Principal, action PolicyAction, _ string) PolicyDecision {
	if action == ActionRead {
		return PolicyDecision{Allow: true}
	}
	return PolicyDecision{Reason: "no write policy configured"}
}

func (DenyAllPolicy) CheckRow(_ context.Context, _ Principal, action PolicyAction, _ PhysicalFeatureKey) PolicyDecision {
	if action == ActionRead {
		return PolicyDecision{Allow: true}
	}
	return PolicyDecision{Reason: "no write policy configured"}
}

func (DenyAllPolicy) CheckPostImage(_ context.Context, _ Principal, _ PhysicalFeatureKey, _ map[string]TypedValue) PolicyDecision {
	return PolicyDecision{Reason: "no write policy configured"}
}

// AllowAllPolicy allows everything. It is for tests and for explicitly
// trusted deployments only; it must never be the default.
type AllowAllPolicy struct{}

func (AllowAllPolicy) CheckCollection(_ context.Context, _ Principal, _ PolicyAction, _ string) PolicyDecision {
	return PolicyDecision{Allow: true}
}
func (AllowAllPolicy) CheckRow(_ context.Context, _ Principal, _ PolicyAction, _ PhysicalFeatureKey) PolicyDecision {
	return PolicyDecision{Allow: true}
}
func (AllowAllPolicy) CheckPostImage(_ context.Context, _ Principal, _ PhysicalFeatureKey, _ map[string]TypedValue) PolicyDecision {
	return PolicyDecision{Allow: true}
}

func policyActionFor(opName string) (PolicyAction, error) {
	switch opName {
	case "insert":
		return ActionInsert, nil
	case "replace":
		return ActionReplace, nil
	case "update":
		return ActionUpdate, nil
	case "delete":
		return ActionDelete, nil
	default:
		return ActionRead, fmt.Errorf("unknown mutation op %q", opName)
	}
}

// CollectionOnlyPolicy explicitly declares that authorization does not depend
// on row contents. Row-dependent policies require transactional image reads,
// which the current mutation provider contract does not offer.
type CollectionOnlyPolicy interface {
	Policy
	CollectionOnly() bool
}

func (AllowAllPolicy) CollectionOnly() bool { return true }
