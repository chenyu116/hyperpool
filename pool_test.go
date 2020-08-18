package hyperpool

import (
	"testing"
)

func BenchmarkPool(b *testing.B) {
	h:=&Pool{
		New:   func() interface{} {
			return 1
		},
		Limit: 2000,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := h.Get()
			_ = e
			h.Put(e)
		}
	})
}
