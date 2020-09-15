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

func NewPool(new func() interface{}, cfg ...PoolConfig) *Pool {
	config := PoolConfig{
		MaxConn:      100,
		MaxKeepConn:  2,
		ReleaseAfter: 120 * time.Second,
	}
	if len(cfg) > 0 {
		config = cfg[0]
	}
	if config.MaxKeepConn <= 0 {
		config.MaxKeepConn = 1
	}
	if config.MaxConn < uint32(config.MaxKeepConn) {
		config.MaxConn = uint32(config.MaxKeepConn)
	}
	p := &Pool{
		Config:        config,
		pools:         make(chan interface{}, config.MaxConn),
		releaseUpdate: make(chan bool, (config.MaxConn+config.MaxConn%2)/2),
		new:           new,
	}
	if new != nil {
		for i := 0; i < config.MaxKeepConn; i++ {
			p.pools <- p.new()
		}
		p.createdConn = int32(config.MaxKeepConn)
	}
	go p.release()
	return p
}

type Pool struct {
	Config        PoolConfig
	new           func() interface{}
	Close         func(x interface{})
	createdConn   int32
	pools         chan interface{}
	isReleasing   int32
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
	if x == nil && p.new != nil {
		if p.createdConn < int32(p.Config.MaxConn) {
			x = p.new()
			if x != nil {
				atomic.AddInt32(&p.createdConn, 1)
			}
		}
	}
	return
}

func (p *Pool) Put(x interface{}) {
	if x == nil {
		return
	}

	if p.releasing() {
		p.Revoke(x)
		return
	}
	select {
	case p.pools <- x:
		if !p.releasing() {
			p.releaseUpdate <- true
		}
	default:
		p.Revoke(x)
	}
}

func (p *Pool) Revoke(x interface{}) {
	if p.Close != nil {
		p.Close(x)
		atomic.AddInt32(&p.createdConn, -1)
	}
}

func (p *Pool) GetCreateConn() int32 {
	return p.createdConn
}

func (p *Pool) releasing() bool {
	return atomic.LoadInt32(&p.isReleasing) == 1
}
func (p *Pool) startRelease() {
	atomic.StoreInt32(&p.isReleasing, 1)
}
func (p *Pool) stopRelease() {
	atomic.StoreInt32(&p.isReleasing, 0)
}
func (p *Pool) release() {
	releaseTimer := time.NewTimer(p.Config.ReleaseAfter)
	defer releaseTimer.Stop()
	var x interface{}
	for {
		select {
		case <-releaseTimer.C:
			if !p.releasing() {
				p.startRelease()
				for len(p.pools) > 0 {
					x = <-p.pools
					p.Revoke(x)
				}
				if p.new != nil {
					for i := 0; i < p.Config.MaxKeepConn; i++ {
						x = p.new()
						if x != nil {
							p.pools <- x
							atomic.AddInt32(&p.createdConn, 1)
						}
					}
				}
				x = nil
				releaseTimer.Reset(p.Config.ReleaseAfter)
				p.stopRelease()
			}
		case <-p.releaseUpdate:
			releaseTimer.Reset(p.Config.ReleaseAfter)
		}
	}
}
