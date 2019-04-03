package hyperpool

import (
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	Pools    [32]interface{}
	free     uint32
	lastPut  uint32
	cleaning int32

	sync.Mutex

	New   func() interface{}
	Close func(x interface{})
}

func (h *Pool) Get() (x interface{}) {
	if atomic.LoadInt32(&h.cleaning) == 1 {
		if h.New != nil {
			x = h.New()
		}
		return
	}
	h.Lock()
	x = h.Pools[0]
	copy(h.Pools[0:31], h.Pools[1:32])
	h.Pools[31] = nil
	h.Unlock()
	if x == nil && h.New != nil {
		x = h.New()
	}
	return
}

func (h *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	if atomic.LoadInt32(&h.cleaning) == 1 {
		if h.Close != nil {
			h.Close(x)
		}
		return
	}

	atomic.StoreUint32(&h.lastPut, 0)

	h.Lock()
	for i := 0; i < 32; i++ {
		if h.Pools[i] == nil {
			h.Pools[i] = x
			h.Unlock()
			return
		}
	}
	h.Unlock()
}

func (h *Pool) clean() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		if atomic.LoadInt32(&h.cleaning) == 1 {
			continue
		}

		if h.free < atomic.LoadUint32(&h.lastPut) {
			atomic.StoreInt32(&h.cleaning, 1)
			h.Lock()
			for i := 0; i < 32; i++ {
				if h.Pools[i] != nil && h.Close != nil {
					h.Close(h.Pools[i])
				}
				h.Pools[i] = nil
			}
			h.Unlock()
			atomic.StoreUint32(&h.lastPut, 0)
			atomic.StoreInt32(&h.cleaning, 0)
			continue
		}
		atomic.AddUint32(&h.lastPut, 1)
	}
}

func NewPool(free uint32) *Pool {
	if free != 0 && free < 60 {
		free = 60
	}

	h := &Pool{free: free}
	if free > 0 {
		go h.clean()
	}

	return h
}
