package util

import (
	"sync/atomic"
	"time"
)

var nowInMs = uint64(0)

// StartTimeTicker 启动一个后台任务，每毫秒缓存当前时间戳，
// 在高并发场景下提供更好的性能.
func StartTimeTicker() { // 每一毫秒，更新一次计数
	atomic.StoreUint64(&nowInMs, uint64(time.Now().UnixNano())/UnixTimeUnitOffset)
	go func() {
		for {
			now := uint64(time.Now().UnixNano()) / UnixTimeUnitOffset // 毫秒
			atomic.StoreUint64(&nowInMs, now)
			time.Sleep(time.Millisecond)
		}
	}()
}

func CurrentTimeMillsWithTicker() uint64 {
	return atomic.LoadUint64(&nowInMs) // 毫秒
}
