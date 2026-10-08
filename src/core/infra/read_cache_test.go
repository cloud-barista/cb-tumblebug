package infra

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/core/common"
	"github.com/cloud-barista/cb-tumblebug/src/core/model"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

// Concurrent identical reads share one fetch; a write invalidates; the entry expires after the TTL.
func TestReadCache_CoalescesInvalidatesAndExpires(t *testing.T) {
	var fetches int32
	key := "infra:t/x"
	readCache.Delete(key)
	fetch := func() (any, error) {
		atomic.AddInt32(&fetches, 1)
		time.Sleep(50 * time.Millisecond) // long enough for the others to pile up on singleflight
		return "v", nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); readCached(key, fetch) }()
	}
	wg.Wait()
	if n := atomic.LoadInt32(&fetches); n != 1 {
		t.Fatalf("20 concurrent reads should fetch once, fetched %d times", n)
	}
	readCached(key, fetch) // within TTL: served from cache
	if n := atomic.LoadInt32(&fetches); n != 1 {
		t.Fatalf("read within TTL should not fetch, fetched %d times", n)
	}
	InvalidateReadCache("t", "x")
	readCached(key, fetch)
	if n := atomic.LoadInt32(&fetches); n != 2 {
		t.Fatalf("read after invalidation should fetch again, fetched %d times", n)
	}
	readCache.Store(key, readCacheEntry{val: "old", exp: time.Now().Add(-time.Millisecond)})
	if v, _ := readCached(key, fetch); v != "v" {
		t.Fatalf("expired entry must be refetched, got %v", v)
	}
}

// UpdateNodeInfo invalidates the Infra's cached detail and the namespace lists.
func TestReadCache_WritePathInvalidates(t *testing.T) {
	cleanup := kvstore.SetTestStore(kvtest.NewMemoryStore())
	defer cleanup()
	node := model.NodeInfo{Id: "n1", Name: "n1", Status: model.StatusRunning}
	b, _ := json.Marshal(node)
	_ = kvstore.Put(common.GenInfraKey("ns", "inf", "n1"), string(b))
	readCache.Store("infra:ns/inf", readCacheEntry{val: "detail", exp: time.Now().Add(time.Hour)})
	readCache.Store("list:ns/status", readCacheEntry{val: "list", exp: time.Now().Add(time.Hour)})
	readCache.Store("list:other/status", readCacheEntry{val: "keep", exp: time.Now().Add(time.Hour)})
	UpdateNodeInfo("ns", "inf", node)
	if _, ok := readCache.Load("infra:ns/inf"); ok {
		t.Fatal("detail cache not invalidated by UpdateNodeInfo")
	}
	if _, ok := readCache.Load("list:ns/status"); ok {
		t.Fatal("list cache not invalidated by UpdateNodeInfo")
	}
	if _, ok := readCache.Load("list:other/status"); !ok {
		t.Fatal("another namespace's list cache must be kept")
	}
}

// The concurrency cap rejects a read once the slots are taken for longer than infraReadWait.
func TestInfraReadSemaphore_Busy(t *testing.T) {
	var held []func()
	for i := 0; i < infraReadConcurrency; i++ {
		rel, err := acquireInfraRead()
		if err != nil {
			t.Fatalf("slot %d: %v", i, err)
		}
		held = append(held, rel)
	}
	start := time.Now()
	if _, err := acquireInfraRead(); err != ErrReadBusy {
		t.Fatalf("expected ErrReadBusy, got %v", err)
	}
	if time.Since(start) < infraReadWait {
		t.Fatal("busy read returned before the wait elapsed")
	}
	held[0]()
	if rel, err := acquireInfraRead(); err != nil {
		t.Fatalf("slot should be free again: %v", err)
	} else {
		rel()
	}
	for _, r := range held[1:] {
		r()
	}
}
