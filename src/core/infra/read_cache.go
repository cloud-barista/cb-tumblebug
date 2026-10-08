package infra

import (
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cloud-barista/cb-tumblebug/src/core/model"
)

// Short-lived read cache for the two heaviest read APIs (Infra detail and Infra list).
// Several GUIs polling the same Infra collapse into one read: concurrent identical requests
// share one fetch (singleflight) and the result is reused for readCacheTTL. Every write path
// (UpdateNodeInfo, UpdateInfraInfo, node create/delete, Infra create/delete) invalidates the
// Infra's entries, so a read right after a control action never returns the stale copy.
const readCacheTTL = time.Second

type readCacheEntry struct {
	val any
	exp time.Time
}

var (
	readCache sync.Map // key -> readCacheEntry
	readGroup singleflight.Group
)

func readCached(key string, fetch func() (any, error)) (any, error) {
	if v, ok := readCache.Load(key); ok && time.Now().Before(v.(readCacheEntry).exp) {
		return v.(readCacheEntry).val, nil
	}
	v, err, _ := readGroup.Do(key, func() (any, error) {
		if v, ok := readCache.Load(key); ok && time.Now().Before(v.(readCacheEntry).exp) {
			return v.(readCacheEntry).val, nil
		}
		val, err := fetch()
		if err == nil {
			readCache.Store(key, readCacheEntry{val: val, exp: time.Now().Add(readCacheTTL)})
		}
		return val, err
	})
	return v, err
}

// InvalidateReadCache drops the cached detail of one Infra and every cached list of its namespace.
func InvalidateReadCache(nsId, infraId string) {
	if infraId != "" {
		readCache.Delete("infra:" + nsId + "/" + infraId)
	}
	prefix := "list:" + nsId + "/"
	readCache.Range(func(k, _ any) bool {
		if strings.HasPrefix(k.(string), prefix) {
			readCache.Delete(k)
		}
		return true
	})
}

// GetInfraInfoCached is GetInfraInfo behind the read cache. Callers must treat the result as
// read-only: it is shared with other readers for up to readCacheTTL.
func GetInfraInfoCached(nsId, infraId string) (*model.InfraInfo, error) {
	v, err := readCached("infra:"+nsId+"/"+infraId, func() (any, error) { return GetInfraInfo(nsId, infraId) })
	if err != nil {
		return nil, err
	}
	return v.(*model.InfraInfo), nil
}

// ListInfraInfoCached is ListInfraInfo behind the read cache (read-only result).
func ListInfraInfoCached(nsId, option string) ([]model.InfraInfoSummary, error) {
	v, err := readCached("list:"+nsId+"/"+option, func() (any, error) { return ListInfraInfo(nsId, option) })
	if err != nil {
		return nil, err
	}
	return v.([]model.InfraInfoSummary), nil
}

// Concurrency cap for the full Infra read (etcd prefix read + per-Node hydration). It bounds
// the load regardless of how many clients poll; the read cache keeps most requests from
// reaching it. ErrReadBusy is returned when the wait exceeds infraReadWait.
const (
	infraReadConcurrency = 8
	infraReadWait        = 3 * time.Second
)

var (
	infraReadSem = make(chan struct{}, infraReadConcurrency)
	ErrReadBusy  = errors.New("too many concurrent Infra reads; retry shortly")
)

func acquireInfraRead() (release func(), err error) {
	select {
	case infraReadSem <- struct{}{}:
		return func() { <-infraReadSem }, nil
	case <-time.After(infraReadWait):
		return nil, ErrReadBusy
	}
}
