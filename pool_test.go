package hyperpool

import (
	"testing"
	"time"
)

//func BenchmarkPool(b *testing.B) {
//	var h = NewPool(10, time.Second*3, func() interface{} {
//		return 1
//	})
//
//	b.RunParallel(func(pb *testing.PB) {
//		for pb.Next() {
//			e := h.Get()
//			_ = e
//			h.Put(e)
//		}
//	})
//	b.Log(h.Len())
//}

func TestNewPool(t *testing.T) {
	h := NewPool(10, time.Second*3, func() interface{} {
		return 1
	})
	t.Log(h)
	time.Sleep(time.Second * 15)
	t.Error(h.Len())
}
