package hyperpool

import (
	"testing"
	"time"
)

func BenchmarkPool(b *testing.B) {
	p := NewPool(func() interface{} {
		return 1
	}, PoolConfig{
		MaxConn:      1000,
		MaxKeepConn:  2,
		ReleaseAfter: time.Second * 10,
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := p.Get()
			p.Put(e)
		}
	})
}
