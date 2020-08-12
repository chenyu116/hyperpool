package hyperpool

import (
	"fmt"
	"github.com/valyala/fastrand"
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	pools    []interface{}
	free     int32
	lastPut  uint32
	cleaning int32

	mu sync.Mutex

	New   func() interface{}
	Close func(x interface{})

	poolLock     []int32
	poolRaceHash []interface{}
	resetAfter   time.Duration
	cacheSize    uint32
}

func (h *Pool) Get() (x interface{}) {
	//if atomic.LoadInt32(&h.cleaning) == 1 {
	//	if h.New != nil {
	//		x = h.New()
	//	}
	//	return
	//}
	pid := fastrand.Uint32n(h.cacheSize)
	if atomic.CompareAndSwapInt32(&h.poolLock[pid], 0, 1) {
		x = h.poolRaceHash[pid]
		h.poolRaceHash[pid] = nil
		if x != nil {
			atomic.AddInt32(&h.free, 1)
		}
	}
	//if x == nil {
	//	h.mu.Lock()
	//	last := len(h.pools) - 1
	//	if last >= 0 {
	//		x = h.pools[last]
	//		h.pools = h.pools[:last]
	//	}
	//	h.mu.Unlock()
	//}
	if x == nil && h.New != nil {
		atomic.AddInt32(&h.cleaning, 1)
		x = h.New()
	}
	return
}

func (h *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	//if atomic.LoadInt32(&h.cleaning) == 1 {
	//	if h.Close != nil {
	//		h.Close(x)
	//	}
	//	return
	//}
	pid := fastrand.Uint32n(h.cacheSize)
	if atomic.LoadInt32(&h.poolLock[pid]) == 1 {
		h.poolRaceHash[pid] = x
		atomic.StoreInt32(&h.poolLock[pid], 0)
		return
	}
	//atomic.StoreUint32(&h.lastPut, 0)
	//
	//h.mu.Lock()
	//h.pools = append(h.pools, x)
	//h.mu.Unlock()
}

func (h *Pool) reset() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		fmt.Println(atomic.LoadInt32(&h.cleaning))
		//if atomic.LoadInt32(&h.cleaning) == 1 {
		//	continue
		//}
		//atomic.StoreInt32(&h.cleaning, 1)
		//if h.free < h.lastPut {
		//	h.mu.Lock()
		//	for i := 0; i < len(h.pools); i++ {
		//		if h.pools[i] != nil && h.Close != nil {
		//			h.Close(h.pools[i])
		//		}
		//		h.pools[i] = nil
		//	}
		//	h.pools = []interface{}{}
		//	h.mu.Unlock()
		//	h.lastPut = 0
		//} else {
		//	h.lastPut += 10
		//}
		//atomic.StoreInt32(&h.cleaning, 0)
	}
}

func NewPool(cacheSize int, resetAfter time.Duration) *Pool {
	h := new(Pool)

	if resetAfter == 0 {
		h.resetAfter = time.Second * 120
	}
	h.cacheSize = uint32(cacheSize)
	h.poolLock = make([]int32, cacheSize)
	h.poolRaceHash = make([]interface{}, cacheSize)

	go h.reset()

	return h
}
