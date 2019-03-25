package hyperpool

import (
	"runtime"
	"testing"
)

func BenchmarkLoopsParallel(b *testing.B) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	pl := NewPool(1, 120)
	pl.New = func() interface{} {
		return 1
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := pl.Get().(int)
			_ = e
			pl.Put(e)
		}
	})
}
