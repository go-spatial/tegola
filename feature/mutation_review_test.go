package feature

import (
	"context"
	"fmt"
	"github.com/alexeydott/tegola/provider"
	"testing"
)

type reviewScopedPolicy struct{ Policy }

func TestReviewRowPolicyRequiresTransactionalImages(t *testing.T) {
	coordinator := MutationCoordinator{PolicyFor: func(string) Policy { return reviewScopedPolicy{AllowAllPolicy{}} }, SchemaFor: func(string) (*SchemaDescriptor, error) {
		t.Fatal("row policy admitted without transactional images")
		return nil, nil
	}}
	_, _, err := coordinator.Execute(context.Background(), Principal{}, provider.Mutation{Op: provider.MutationDelete, Collection: "sites", FeatureID: 1})
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrUnsupportedCapability {
		t.Fatalf("want unsupported capability; got %v", err)
	}
}

type reviewCommitProvider struct{ rolledBack bool }

func (p *reviewCommitProvider) DescribeWritable(context.Context, string) (provider.WriteDescriptor, error) {
	return provider.WriteDescriptor{Domain: "same"}, nil
}
func (p *reviewCommitProvider) BeginFeatureTx(context.Context, provider.TxOptions) (provider.FeatureTx, error) {
	return p, nil
}
func (p *reviewCommitProvider) Apply(_ context.Context, m provider.Mutation) (provider.MutationOutcome, error) {
	return provider.MutationOutcome{FeatureID: m.FeatureID}, nil
}
func (p *reviewCommitProvider) Commit(context.Context) (provider.CommitReceipt, error) {
	return provider.CommitReceipt{Status: provider.CommitCommitted, TransactionID: "confirmed"}, fmt.Errorf("auxiliary failure")
}
func (p *reviewCommitProvider) Rollback(context.Context) error { p.rolledBack = true; return nil }
func TestReviewConfirmedCommitPreservesOutcomesAndInvalidates(t *testing.T) {
	p := &reviewCommitProvider{}
	var invalidated []string
	c := MutationCoordinator{PolicyFor: func(string) Policy { return AllowAllPolicy{} }, SchemaFor: func(string) (*SchemaDescriptor, error) { return &SchemaDescriptor{}, nil }, ProviderFor: func(string) (provider.MutationProvider, string, error) { return p, "physical", nil }, OnCommit: func(cols []string) { invalidated = cols }}
	input := []provider.Mutation{{Op: provider.MutationDelete, Collection: "alias1", FeatureID: 1}, {Op: provider.MutationDelete, Collection: "alias2", FeatureID: 2}}
	outcomes, receipt, err := c.ExecuteAll(context.Background(), Principal{}, input)
	if err == nil || len(outcomes) != 2 || receipt.Status != provider.CommitCommitted || p.rolledBack {
		t.Fatalf("lost commit truth: %+v %+v %v rollback=%v", outcomes, receipt, err, p.rolledBack)
	}
	if len(invalidated) != 2 || invalidated[0] != "alias1" || invalidated[1] != "alias2" {
		t.Fatalf("lost aliases: %v", invalidated)
	}
	if input[0].Collection != "alias1" {
		t.Fatal("mutated caller commands")
	}
}
