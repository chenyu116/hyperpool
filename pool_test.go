package hyperpool

import (
	"testing"
	"time"
)

var h = NewPool(2, time.Second*3, func() interface{} {
	return 1
})

func BenchmarkPool(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e := h.Get()
			_ = e
			h.Put(e)
		}
	})
}
