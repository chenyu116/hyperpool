package hyperpool

import (
	"testing"
	"time"
)

func BenchmarkPool(b *testing.B) {
	p := NewPool(PoolConfig{
		MaxConn:      1000,
		MaxKeepConn:  2,
		ReleaseAfter: time.Second * 10,
	})
	p.New = func() interface{} {
		return 1
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := p.Get()
			p.Put(e)
		}
	})
}
