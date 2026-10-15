package server

import (
	"net/http"

	"github.com/alexeydott/tegola/provider"
)

// writeMutationReceiptHeaders exposes only outcome and correlation metadata.
// Actor and affected collections remain private to the audit store.
func writeMutationReceiptHeaders(w http.ResponseWriter, receipt provider.CommitReceipt) {
	status := "unknown"
	switch receipt.Status {
	case provider.CommitCommitted:
		status = "committed"
	case provider.CommitNotCommitted:
		status = "not-committed"
	}
	w.Header().Set("Tegola-Commit-Status", status)
	mergeFeatureHeader(w.Header(), "Access-Control-Expose-Headers", "Tegola-Commit-Status")
	if receipt.TransactionID != "" {
		w.Header().Set("Tegola-Transaction-Id", receipt.TransactionID)
		mergeFeatureHeader(w.Header(), "Access-Control-Expose-Headers", "Tegola-Transaction-Id")
	}
}
