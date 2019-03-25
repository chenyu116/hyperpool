package hyperpool

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type Pool struct {
	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal
	localSize int            // size of the local array

	lastGet      int64
	freeInterval int64
	isFreeing    int64

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New   func() interface{}
	Close func(x interface{})
}

// Local per-P Pool appendix.
type poolLocalInternal struct {
	private chan interface{} // Can be used only by the respective P.
	shared  []interface{}    // Can be used by any P.

	sync.Mutex // Protects shared.
}

type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}
	if atomic.LoadInt64(&p.isFreeing) == 1 {
		p.Close(x)
		return
	}
	l, _ := p.pin()
	if len(l.private) == 0 {
		l.private <- x
		return
	}

	//fmt.Println("put shared")
	l.Lock()
	l.shared = append(l.shared, x)
	l.Unlock()
}

func (p *Pool) Get() interface{} {
	if atomic.LoadInt64(&p.isFreeing) == 1 {
		return p.New()
	}
	atomic.StoreInt64(&p.lastGet, time.Now().Unix())
	l, pid := p.pin()

	if len(l.private) > 0 {
		//fmt.Println("get channel")
		return <-l.private
	}
	var x interface{}

	l.Lock()
	last := len(l.shared) - 1
	if last >= 0 {
		x = l.shared[last]
		l.shared = l.shared[:last]
		if x != nil {
			//fmt.Println("get shared")
			l.Unlock()
			return x
		}
	}
	l.Unlock()

	if x == nil && p.localSize > 1 {
		x = p.getSlow(pid)
		if x != nil {
			//fmt.Println("get slow")
			return x
		}

	}
	if x == nil && p.New != nil {
		//fmt.Println("get new")
		x = p.New()
	}

	return x
	//}
}

func (p *Pool) getSlow(pid int) (x interface{}) {
	if atomic.LoadInt64(&p.isFreeing) == 1 {
		return nil
	}
	// See the comment in pin regarding ordering of the loads.
	local := p.local // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < p.localSize; i++ {
		if i == pid {
			continue
		}
		l := indexLocal(local, i)
		l.Lock()
		last := len(l.shared) - 1
		if last >= 0 {
			x = l.shared[last]
			l.shared = l.shared[:last]
			l.Unlock()
			return x
		}
		l.Unlock()
	}
	return x
}

func getPid(size int) int {
	if size > 1 {
		return int(Uint32n(uint32(size)))
	}
	return 0
}

// pin pins the current goroutine to P, disables preemption and returns poolLocal pool for the P.
// Caller must call unpin() when done with the pool.
func (p *Pool) pin() (*poolLocal, int) {
	pid := getPid(p.localSize)
	l := p.local // load-consume
	return indexLocal(l, pid), pid
}

func NewPool(length int, freeInterval int64) *Pool {
	p := &Pool{
		freeInterval: freeInterval,
	}

	p.localSize = length
	if p.localSize <= 0 {
		p.localSize = 1
	}

	local := make([]poolLocal, p.localSize)
	startPointer := unsafe.Pointer(&local[0])
	atomic.StorePointer(&p.local, startPointer) // store-release

	for i := 0; i < p.localSize; i++ {
		((*poolLocal)(unsafe.Pointer(uintptr(startPointer) + uintptr(i)*unsafe.Sizeof(poolLocal{})))).private = make(chan interface{}, 1)
	}

	go p.poolCleanup()

	return p
}

func (p *Pool) poolCleanup() {
	if p.freeInterval == 0 {
		return
	}
	_size := p.localSize

	freeTicker := time.NewTicker(time.Second * 10)
	for c := range freeTicker.C {
		if atomic.LoadInt64(&p.isFreeing) == 1 {
			continue
		}
		if (c.Unix() - atomic.LoadInt64(&p.lastGet)) > p.freeInterval {
			atomic.StoreInt64(&p.isFreeing, 1)
			for i := 0; i < _size; i++ {
				l := indexLocal(p.local, i)
				l.Lock()
				for len(l.private) > 0 {
					p.Close(<-l.private)
				}
				for j := range l.shared {
					p.Close(l.shared[j])
				}
				l.shared = nil
				l.Unlock()
			}
			atomic.StoreInt64(&p.isFreeing, 0)
		}
	}
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

func Uint32() uint32 {
	v := rngPool.Get()
	if v == nil {
		v = &rng{}
	}
	r := v.(*rng)
	x := r.Uint32()
	rngPool.Put(r)
	return x
}

var rngPool sync.Pool

// Uint32n returns pseudo-random uint32 in the range [0..maxN).
//
// It is safe calling this function from concurrent goroutines.
func Uint32n(maxN uint32) uint32 {
	x := Uint32()
	// See http://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return uint32((uint64(x) * uint64(maxN)) >> 32)
}

// RNG is a pseudo-random number generator.
//
// It is unsafe to call RNG methods from concurrent goroutines.
type rng struct {
	x uint32
}

// Uint32 returns pseudo-random uint32.
//
// It is unsafe to call this method from concurrent goroutines.
func (r *rng) Uint32() uint32 {
	for r.x == 0 {
		r.x = getRandomUint32()
	}

	// See https://en.wikipedia.org/wiki/Xorshift
	x := r.x
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	r.x = x
	return x
}

func getRandomUint32() uint32 {
	x := time.Now().UnixNano()
	return uint32((x >> 32) ^ x)
}
