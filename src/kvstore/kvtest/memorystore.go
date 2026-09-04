package kvtest

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// MemoryStore is an in-memory implementation of kvstore.Store for unit tests.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMemoryStore creates a new in-memory kvstore.Store instance.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]string),
	}
}

func (m *MemoryStore) NewSession(ctx context.Context) (*concurrency.Session, error) {
	return nil, nil
}

func (m *MemoryStore) NewLock(ctx context.Context, session *concurrency.Session, lockKey string) (*concurrency.Mutex, error) {
	return nil, nil
}

func (m *MemoryStore) Put(key, value string) error {
	return m.PutWith(context.Background(), key, value)
}

func (m *MemoryStore) PutWith(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *MemoryStore) Get(key string) (string, bool, error) {
	return m.GetWith(context.Background(), key)
}

func (m *MemoryStore) GetWith(ctx context.Context, key string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *MemoryStore) GetList(keyPrefix string) ([]string, error) {
	return m.GetListWith(context.Background(), keyPrefix)
}

func (m *MemoryStore) GetListWith(ctx context.Context, keyPrefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []string
	for k, v := range m.data {
		if strings.HasPrefix(k, keyPrefix) {
			results = append(results, v)
		}
	}
	sort.Strings(results)
	return results, nil
}

func (m *MemoryStore) GetKv(key string) (kvstore.KeyValue, bool, error) {
	return m.GetKvWith(context.Background(), key)
}

func (m *MemoryStore) GetKvWith(ctx context.Context, key string) (kvstore.KeyValue, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	if !ok {
		return kvstore.KeyValue{}, false, nil
	}
	return kvstore.KeyValue{Key: key, Value: val}, true, nil
}

func (m *MemoryStore) GetKvList(keyPrefix string) ([]kvstore.KeyValue, error) {
	return m.GetKvListWith(context.Background(), keyPrefix)
}

func (m *MemoryStore) GetKvListWith(ctx context.Context, keyPrefix string) ([]kvstore.KeyValue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []kvstore.KeyValue
	for k, v := range m.data {
		if strings.HasPrefix(k, keyPrefix) {
			results = append(results, kvstore.KeyValue{Key: k, Value: v})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Key < results[j].Key
	})
	return results, nil
}

func (m *MemoryStore) GetKeyList(keyPrefix string) ([]string, error) {
	return m.GetKeyListWith(context.Background(), keyPrefix)
}

func (m *MemoryStore) GetKeyListWith(ctx context.Context, keyPrefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var keys []string
	for k := range m.data {
		if strings.HasPrefix(k, keyPrefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *MemoryStore) GetSortedKvList(keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	return m.GetSortedKvListWith(context.Background(), keyPrefix, sortBy, order)
}

func (m *MemoryStore) GetSortedKvListWith(ctx context.Context, keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	return m.GetKvListWith(ctx, keyPrefix)
}

func (m *MemoryStore) GetKvMap(keyPrefix string) (kvstore.KeyValueMap, error) {
	return m.GetKvMapWith(context.Background(), keyPrefix)
}

func (m *MemoryStore) GetKvMapWith(ctx context.Context, keyPrefix string) (kvstore.KeyValueMap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make(kvstore.KeyValueMap)
	for k, v := range m.data {
		if strings.HasPrefix(k, keyPrefix) {
			res[k] = v
		}
	}
	return res, nil
}

func (m *MemoryStore) Delete(key string) error {
	return m.DeleteWith(context.Background(), key)
}

func (m *MemoryStore) DeleteWith(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MemoryStore) DeleteWithPrefix(keyPrefix string) error {
	return m.DeleteWithPrefixWith(context.Background(), keyPrefix)
}

func (m *MemoryStore) DeleteWithPrefixWith(ctx context.Context, keyPrefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.data {
		if strings.HasPrefix(k, keyPrefix) {
			delete(m.data, k)
		}
	}
	return nil
}

func (m *MemoryStore) WatchKey(key string) clientv3.WatchChan {
	return nil
}

func (m *MemoryStore) WatchKeyWith(ctx context.Context, key string) clientv3.WatchChan {
	return nil
}

func (m *MemoryStore) WatchKeys(keyPrefix string) clientv3.WatchChan {
	return nil
}

func (m *MemoryStore) WatchKeysWith(ctx context.Context, keyPrefix string) clientv3.WatchChan {
	return nil
}

func (m *MemoryStore) Compact(ctx context.Context) error {
	return nil
}

func (m *MemoryStore) Defragment(ctx context.Context) error {
	return nil
}

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]string)
	return nil
}
