package integration_test

import (
	"context"
	"sync"
)

type countingBatchReceiver struct {
	mu             sync.Mutex
	cycleSizes     []int
	bulkOps        int
	processedTotal int
	failNext       bool
}

func (r *countingBatchReceiver) BatchReceive(_ context.Context, state any, payloads []any) (any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failNext {
		r.failNext = false
		return state, errBatchReceiverFail
	}
	r.cycleSizes = append(r.cycleSizes, len(payloads))
	r.bulkOps++
	r.processedTotal += len(payloads)
	count := 0
	if state != nil {
		count = state.(int)
	}
	return count + len(payloads), nil
}

func (r *countingBatchReceiver) stats() (sizes []int, bulkOps, processed int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sizes = append([]int(nil), r.cycleSizes...)
	return sizes, r.bulkOps, r.processedTotal
}

func (r *countingBatchReceiver) triggerFailure() {
	r.mu.Lock()
	r.failNext = true
	r.mu.Unlock()
}
