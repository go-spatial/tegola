package feature

import (
	"context"
	"errors"
	"github.com/alexeydott/tegola/provider"
	"testing"
)

type receiptProbe struct {
	status                        provider.CommitStatus
	beginErr, applyErr, commitErr error
	committed, rolledBack         int
}

func (p *receiptProbe) DescribeWritable(context.Context, string) (provider.WriteDescriptor, error) {
	return provider.WriteDescriptor{Domain: "test"}, nil
}
func (p *receiptProbe) BeginFeatureTx(context.Context, provider.TxOptions) (provider.FeatureTx, error) {
	return p, p.beginErr
}
func (p *receiptProbe) Apply(_ context.Context, m provider.Mutation) (provider.MutationOutcome, error) {
	return provider.MutationOutcome{FeatureID: m.FeatureID}, p.applyErr
}
func (p *receiptProbe) Commit(context.Context) (provider.CommitReceipt, error) {
	p.committed++
	return provider.CommitReceipt{Status: p.status, TransactionID: "operation-42"}, p.commitErr
}
func (p *receiptProbe) Rollback(context.Context) error { p.rolledBack++; return nil }
func receiptCoordinator(p *receiptProbe) *MutationCoordinator {
	return &MutationCoordinator{
		PolicyFor:   func(string) Policy { return AllowAllPolicy{} },
		SchemaFor:   func(string) (*SchemaDescriptor, error) { return &SchemaDescriptor{}, nil },
		ProviderFor: func(string) (provider.MutationProvider, string, error) { return p, "storage", nil },
	}
}
func receiptMutation() provider.Mutation {
	return provider.Mutation{Op: provider.MutationDelete, Collection: "sites", FeatureID: 42}
}

func TestReceiptPreCommitFailureIsNotUnknown(t *testing.T) {
	for _, stage := range []string{"empty", "policy", "schema", "provider", "begin", "lock", "apply"} {
		t.Run(stage, func(t *testing.T) {
			failure := errors.New("failure before commit")
			p := &receiptProbe{status: provider.CommitCommitted}
			c := receiptCoordinator(p)
			input := []provider.Mutation{receiptMutation()}
			switch stage {
			case "empty":
				input = nil
			case "policy":
				c.PolicyFor = func(string) Policy { return DenyAllPolicy{} }
			case "schema":
				c.SchemaFor = func(string) (*SchemaDescriptor, error) { return nil, failure }
			case "provider":
				c.ProviderFor = func(string) (provider.MutationProvider, string, error) { return nil, "", failure }
			case "begin":
				p.beginErr = failure
			case "lock":
				c.LockCheck = func(context.Context, string, PhysicalFeatureKey) error { return failure }
			case "apply":
				p.applyErr = failure
			}
			out, receipt, err := c.ExecuteAll(context.Background(), Principal{}, input)
			if err == nil || len(out) != 0 || receipt.Status != provider.CommitNotCommitted || p.committed != 0 {
				t.Fatalf("stage %s: receipt=%+v outcomes=%v err=%v commits=%d", stage, receipt, out, err, p.committed)
			}
			if (stage == "apply" || stage == "lock") && p.rolledBack != 1 {
				t.Fatalf("rollback=%d", p.rolledBack)
			}
		})
	}
}
func TestReceiptNonCommittedNeverInvalidates(t *testing.T) {
	for _, status := range []provider.CommitStatus{provider.CommitUnknown, provider.CommitNotCommitted} {
		p := &receiptProbe{status: status}
		c := receiptCoordinator(p)
		calls := 0
		c.OnCommit = func([]string) { calls++ }
		out, receipt, err := c.ExecuteAll(context.Background(), Principal{}, []provider.Mutation{receiptMutation()})
		if err == nil || len(out) != 0 || receipt.Status != status || receipt.TransactionID != "operation-42" || calls != 0 {
			t.Fatalf("status=%v receipt=%+v out=%v err=%v calls=%d", status, receipt, out, err, calls)
		}
	}
}
func TestReceiptCommittedAuxiliaryErrorKeepsSingleOutcome(t *testing.T) {
	failure := errors.New("auxiliary failure")
	p := &receiptProbe{status: provider.CommitCommitted, commitErr: failure}
	out, receipt, err := receiptCoordinator(p).Execute(context.Background(), Principal{}, receiptMutation())
	if out.FeatureID != 42 || receipt.Status != provider.CommitCommitted || !errors.Is(err, failure) || p.rolledBack != 0 {
		t.Fatalf("out=%+v receipt=%+v err=%v rollback=%d", out, receipt, err, p.rolledBack)
	}
}
func TestReceiptCallbackPanicKeepsCommit(t *testing.T) {
	p := &receiptProbe{status: provider.CommitCommitted}
	c := receiptCoordinator(p)
	c.OnCommit = func([]string) { panic("cache callback failed") }
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("callback panic escaped confirmed commit: %v", r)
		}
	}()
	out, receipt, err := c.Execute(context.Background(), Principal{ID: "actor"}, receiptMutation())
	if out.FeatureID != 42 || receipt.Status != provider.CommitCommitted || receipt.TransactionID != "operation-42" || err == nil || p.rolledBack != 0 {
		t.Fatalf("out=%+v receipt=%+v err=%v rollback=%d", out, receipt, err, p.rolledBack)
	}
}

func TestReceiptCallbackPanicPreservesProviderAuxiliaryError(t *testing.T) {
	failure := errors.New("provider auxiliary error")
	p := &receiptProbe{status: provider.CommitCommitted, commitErr: failure}
	c := receiptCoordinator(p)
	c.OnCommit = func(cols []string) { cols[0] = "changed"; panic("notification failed") }
	out, receipt, err := c.Execute(context.Background(), Principal{}, receiptMutation())
	if out.FeatureID != 42 || !errors.Is(err, failure) || receipt.Collections[0] != "sites" || p.rolledBack != 0 {
		t.Fatalf("lost committed state: %+v %+v %v", out, receipt, err)
	}
}

func TestReceiptInvalidatesCommittedAndUnknownOnly(t *testing.T) {
	for _, status := range []provider.CommitStatus{provider.CommitCommitted, provider.CommitUnknown, provider.CommitNotCommitted} {
		p := &receiptProbe{status: status, commitErr: errors.New("provider error")}
		c := receiptCoordinator(p)
		calls := 0
		c.OnInvalidate = func(cols []string) {
			calls++
			if len(cols) != 1 || cols[0] != "sites" {
				t.Fatalf("collections=%v", cols)
			}
			cols[0] = "mutated"
		}
		_, receipt, _ := c.Execute(context.Background(), Principal{}, receiptMutation())
		want := 0
		if status != provider.CommitNotCommitted {
			want = 1
		}
		if calls != want || receipt.Status != status {
			t.Fatalf("status=%v calls=%d receipt=%+v", status, calls, receipt)
		}
		if status == provider.CommitCommitted && receipt.Collections[0] != "sites" {
			t.Fatal("callback mutated receipt")
		}
	}
}
