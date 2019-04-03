package hyperpool

import (
	"testing"
)

func BenchmarkPool(b *testing.B) {
	h := NewPool(120)
	h.New = func() interface{} {
		return 1
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := h.Get()
			_ = e
			h.Put(e)
		}
	})
}
