package hyperpool

import (
	"testing"
	"time"
)

func BenchmarkPool(b *testing.B) {
	h := &Pool{
		New: func() interface{} {
			return 1
		},
		Limit: 2000,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := h.Get(time.Millisecond * 300)
			_ = e
			h.Put(e)
		}
	})
}
