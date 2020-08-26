package hyperpool

import (
	"sync/atomic"
	"time"
)

type PoolConfig struct {
	// max connections limit
	MaxConn uint32
	// max reCreate connections after release
	MaxKeepConn int
	// after this time if there was no client to put connection, Pool will release the connections
	ReleaseAfter time.Duration
}

func NewPool(cfg ...PoolConfig) *Pool {
	config := PoolConfig{
		MaxConn:      100,
		MaxKeepConn:  2,
		ReleaseAfter: 120 * time.Second,
	}
	if len(cfg) > 0 {
		config = cfg[0]
	}
	p := &Pool{
		Config:        config,
		gets:          0,
		pools:         make(chan interface{}, config.MaxConn),
		releaseUpdate: make(chan bool, (config.MaxConn+config.MaxConn%2)/2),
	}
	go p.release()
	return p
}

type Pool struct {
	Config        PoolConfig
	New           func() interface{}
	Close         func(x interface{})
	gets          int32
	pools         chan interface{}
	isReleasing bool
	releaseUpdate chan bool
}

func (p *Pool) Get(wait ...time.Duration) (x interface{}) {
	select {
	case x = <-p.pools:
	default:
	}
	if x == nil && len(wait) > 0 && wait[0] > 0 {
		select {
		case <-time.After(wait[0]):
		case x = <-p.pools:
		}
	}
	if x != nil {
		atomic.AddInt32(&p.gets, 1)
	}
	if x == nil && p.New != nil {
		if p.gets < int32(p.Config.MaxConn) {
			x = p.New()
			atomic.AddInt32(&p.gets, 1)
		}
	}
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}

	if p.isReleasing {
		if p.Close != nil {
			p.Close(x)
		}
		if p.New != nil && p.gets > 0 {
			atomic.AddInt32(&p.gets, -1)
		}
		return
	}
	select {
	case p.pools <- x:
		if p.New != nil && p.gets > 0 {
			atomic.AddInt32(&p.gets, -1)
		}
		if !p.isReleasing {
			p.releaseUpdate <- true
		}
	default:
		if p.Close != nil {
			p.Close(x)
		}
	}
}

func (p *Pool) release() {
	releaseTimer := time.NewTimer(p.Config.ReleaseAfter)
	defer releaseTimer.Stop()
	var x interface{}
	for {
		select {
		case <-releaseTimer.C:
			p.isReleasing = true
			for {
				if len(p.pools) == 0 {
					x = nil
					break
				}
				x = <-p.pools
				if p.Close != nil {
					p.Close(x)
				}
				atomic.AddInt32(&p.gets, 1)
			}
			if p.New != nil {
				for i := 0; i < p.Config.MaxKeepConn; i++ {
					p.pools <- p.New()
				}
				atomic.AddInt32(&p.gets, -int32(p.Config.MaxKeepConn))
			}
			p.isReleasing = false
		case <-p.releaseUpdate:
			releaseTimer.Reset(p.Config.ReleaseAfter)
		}
	}
}
