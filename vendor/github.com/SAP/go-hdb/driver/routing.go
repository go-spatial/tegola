package driver

import (
	"container/heap"
	"sync"
	"sync/atomic"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

type routingUpdate struct {
	version int64
	rn      p.RoutingNodes
}

// routingNode holds the routing state of a topology node.
type routingNode struct {
	rn        *p.RoutingNode // topology data
	reachable bool
	accessCnt uint64 // access counter
}

// priorityQueue holds pointers to routing state records: pick and setReachable
// mutate reachable/accessCnt in place and reorder the heap in O(log n) without
// copying records or tracking indices.
type priorityQueue []*routingNode

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// higher priority first: reachable nodes before unreachable, primary nodes
	// before standby nodes.
	switch {
	case pq[i].reachable != pq[j].reachable:
		return pq[i].reachable
	case pq[i].rn.Standby != pq[j].rn.Standby:
		return !pq[i].rn.Standby
	default:
		return pq[i].accessCnt < pq[j].accessCnt
	}
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x any) {
	*pq = append(*pq, x.(*routingNode))
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	x := old[n-1]
	old[n-1] = nil // don't stop the GC from reclaiming the item eventually
	*pq = old[0 : n-1]
	return x
}

// routing holds the connector's view of connection routing.
type routing struct {
	update  atomic.Pointer[routingUpdate]
	enabled atomic.Bool

	mu sync.Mutex

	version int64
	// according to the C++ ref impl databaseName+systemID identify a HANA cluster.
	databaseName string
	systemID     string

	last p.RoutingNodeList // last applied topology, see updateNodes
	// states holds the routing state records sorted by state identity
	// (SiteVolumeID, SiteType); updateNodes binary-searches it to relink state.
	// !!! states and pq reference the same records.
	states []*routingNode
	pq     priorityQueue
}

// pick returns the next host to dial.
func (r *routing) pick(fallback string) string {
	if !r.enabled.Load() {
		return fallback
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	update := r.update.Swap(nil)
	if update != nil && update.version == r.version {
		if snl, err := update.rn.SortedNodeList(nil); err == nil {
			r.updateNodes(snl)
		}
	}

	if r.pq.Len() == 0 {
		return fallback
	}

	x := heap.Pop(&r.pq)
	node := x.(*routingNode)
	node.accessCnt++
	heap.Push(&r.pq, node)
	return node.rn.Host
}

// updateFromConnect updates the routing state negotiated on a connect reply and returns the routing version.
func (r *routing) updateFromConnect(enabled bool, host string, databaseName, systemID string, rn p.RoutingNodes) int64 {
	if !enabled {
		r.enabled.Store(false)
		return -1
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	snl, err := rn.SortedNodeList(&host)
	if err != nil {
		r.enabled.Store(false)
		return -1
	}

	// if a change in cluster id we increment the version to detect stale updates from running connections.
	if databaseName != r.databaseName || systemID != r.systemID {
		r.version++
		r.databaseName = databaseName
		r.systemID = systemID
	}
	r.updateNodes(snl)

	r.enabled.Store(true)

	return r.version
}

// updateFromReply stores the routing nodes received on an execute reply for lazy processing.
func (r *routing) updateFromReply(version int64, rn p.RoutingNodes) {
	r.update.Store(&routingUpdate{version: version, rn: rn})
}

// updateNodes applies an incoming topology to the routing state.
// The topology is sorted by state identity (SiteVolumeID, SiteType), see SortedNodeList.
// If equals the last one applied, nothing changes.
// On a change, state records (reachability, access count) of nodes that
// remain part of the topology are preserved and the queue is rebuilt.
// According to the C++ ref impl state is keyed by site identity (see above), not by address:
// .a record whose SiteVolumeID and SiteType match keeps its state even if its dial address changed
// .a record with a new identity starts reachable.
func (r *routing) updateNodes(nodes p.RoutingNodeList) {
	if r.last.Equal(nodes) {
		return
	}
	r.last = nodes
	if len(nodes) == 0 {
		r.states = nil
		r.pq = nil
		return
	}

	old := r.states
	r.states = r.pq[:0] // reuse existing backing arrays, round-robin between states and pq
	i := 0
	for _, node := range nodes {
		// skip old identities smaller than node's
		for i < len(old) && old[i].rn.Compare(node) < 0 {
			i++
		}
		if i < len(old) && old[i].rn.Compare(node) == 0 {
			state := old[i]
			i++
			state.rn = node // same identity: take over the state record
			r.states = append(r.states, state)
		} else {
			r.states = append(r.states, &routingNode{rn: node, reachable: true})
		}
	}
	old = append(old[:0], r.states...)
	r.pq = old
	heap.Init(&r.pq)
}

// setReachable marks all nodes with the host as reachable or unreachable.
func (r *routing) setReachable(host string, reachable bool) {
	if !r.enabled.Load() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	changed := false
	for _, node := range r.pq {
		if node.rn.Host == host && node.reachable != reachable {
			node.reachable = reachable
			changed = true
		}
	}
	if changed {
		heap.Init(&r.pq)
	}
}
