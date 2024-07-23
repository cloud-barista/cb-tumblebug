package kvstore

import (
	"context"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

type KeyValue struct {
	Key   string
	Value string
}

// KeyValueMap represents a key-value pair.
type KeyValueMap map[string]string

// Store defines operations as an interface for key-value store.
// This was mainly implemented for etcd, but can be extended to other key-value stores later.
type Store interface {
	NewSession(ctx context.Context) (*concurrency.Session, error)
	NewLock(ctx context.Context, session *concurrency.Session, lockKey string) (*concurrency.Mutex, error)
	Put(key, value string) error
	PutWith(ctx context.Context, key, value string) error
	Get(key string) (string, error)
	GetWith(ctx context.Context, key string) (string, error)
	GetList(keyPrefix string) ([]string, error)
	GetListWith(ctx context.Context, keyPrefix string) ([]string, error)
	GetKv(key string) (KeyValue, error)
	GetKvWith(ctx context.Context, key string) (KeyValue, error)
	GetKvList(keyPrefix string) ([]KeyValue, error)
	GetKvListWith(ctx context.Context, keyPrefix string) ([]KeyValue, error)
	GetSortedKvList(keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]KeyValue, error)
	GetSortedKvListWith(ctx context.Context, keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]KeyValue, error)
	GetKvMap(keyPrefix string) (KeyValueMap, error)
	GetKvMapWith(ctx context.Context, keyPrefix string) (KeyValueMap, error)
	Delete(key string) error
	DeleteWith(ctx context.Context, key string) error
	WatchKey(key string) clientv3.WatchChan
	WatchKeyWith(ctx context.Context, key string) clientv3.WatchChan
	WatchKeys(keyPrefix string) clientv3.WatchChan
	WatchKeysWith(ctx context.Context, keyPrefix string) clientv3.WatchChan
	Close() error
	// CloseSession(session *concurrency.Session) error
	// Unlock(ctx context.Context, mutex *concurrency.Mutex) error
}
