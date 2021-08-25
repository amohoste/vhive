package snapshotting

type SnapHeap []*Snapshot

func (h SnapHeap) Len() int {
	return len(h)
}
func (h SnapHeap) Less(i, j int) bool {
	return h[i].score < h[j].score
}
func (h SnapHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *SnapHeap) Push(x interface{}) {
	*h = append(*h, x.(*Snapshot))
}

func (h *SnapHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func (h *SnapHeap) Peek() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	return x
}