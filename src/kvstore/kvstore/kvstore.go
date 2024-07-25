package kvstore

import (
	"context"
	"fmt"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// Extensibility: Abstraction and Polymorphism
// Store interface for key-value operations, designed for extensibility.
// Initially for etcd, but adaptable to other stores.

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

type KeyValue struct {
	Key   string
	Value string
}

// KeyValueMap represents a key-value pair.
type KeyValueMap map[string]string

var (
	globalStore Store
	initOnce    sync.Once
)

// Package-level implementation for global Store management
// Provides functions to initialize, access, and manipulate the global Store instance.
// Ensures thread-safe operations and simplifies key-value store interactions.

// InitializeStore initializes the global Store with the provided Store implementation
func InitializeStore(store Store) error {
	if store == nil {
		return fmt.Errorf("provided store is nil")
	}
	initOnce.Do(func() {
		globalStore = store
	})
	return nil
}

// getStore returns the initialized global Store
func getStore() (Store, error) {
	if globalStore == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return globalStore, nil
}

// NewSession creates a new session
func NewSession(ctx context.Context) (*concurrency.Session, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.NewSession(ctx)
}

// NewLock creates a new lock
func NewLock(ctx context.Context, session *concurrency.Session, lockKey string) (*concurrency.Mutex, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.NewLock(ctx, session, lockKey)
}

// Put stores a key-value pair
func Put(key, value string) error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return store.Put(key, value)
}

// PutWith stores a key-value pair with context
func PutWith(ctx context.Context, key, value string) error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return store.PutWith(ctx, key, value)
}

// Get retrieves a value for a given key
func Get(key string) (string, error) {
	store, err := getStore()
	if err != nil {
		return "", err
	}
	return store.Get(key)
}

// GetWith retrieves a value for a given key with context
func GetWith(ctx context.Context, key string) (string, error) {
	store, err := getStore()
	if err != nil {
		return "", err
	}
	return store.GetWith(ctx, key)
}

// GetList retrieves multiple values for keys with the given prefix
func GetList(keyPrefix string) ([]string, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetList(keyPrefix)
}

// GetListWith retrieves multiple values for keys with the given prefix with context
func GetListWith(ctx context.Context, keyPrefix string) ([]string, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetListWith(ctx, keyPrefix)
}

// GetKv retrieves a key-value pair
func GetKv(key string) (KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return KeyValue{}, err
	}
	return store.GetKv(key)
}

// GetKvWith retrieves a key-value pair with context
func GetKvWith(ctx context.Context, key string) (KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return KeyValue{}, err
	}
	return store.GetKvWith(ctx, key)
}

// GetKvList retrieves multiple key-value pairs with the given prefix
func GetKvList(keyPrefix string) ([]KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetKvList(keyPrefix)
}

// GetKvListWith retrieves multiple key-value pairs with the given prefix with context
func GetKvListWith(ctx context.Context, keyPrefix string) ([]KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetKvListWith(ctx, keyPrefix)
}

// GetSortedKvList retrieves sorted key-value pairs with the given prefix
func GetSortedKvList(keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetSortedKvList(keyPrefix, sortBy, order)
}

// GetSortedKvListWith retrieves sorted key-value pairs with the given prefix with context
func GetSortedKvListWith(ctx context.Context, keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]KeyValue, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetSortedKvListWith(ctx, keyPrefix, sortBy, order)
}

// GetKvMap retrieves a map of key-value pairs with the given prefix
func GetKvMap(keyPrefix string) (KeyValueMap, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetKvMap(keyPrefix)
}

// GetKvMapWith retrieves a map of key-value pairs with the given prefix with context
func GetKvMapWith(ctx context.Context, keyPrefix string) (KeyValueMap, error) {
	store, err := getStore()
	if err != nil {
		return nil, err
	}
	return store.GetKvMapWith(ctx, keyPrefix)
}

// Detete removes a key-value pair
func Delete(key string) error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return store.Delete(key)
}

// DeleteWith removes a key-value pair with context
func DeleteWith(ctx context.Context, key string) error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return store.DeleteWith(ctx, key)
}

// WatchKey watches for changes on a specific key
func WatchKey(key string) clientv3.WatchChan {
	store, err := getStore()
	if err != nil {
		return nil
	}
	return store.WatchKey(key)
}

// WatchKeyWith watches for changes on a specific key with context
func WatchKeyWith(ctx context.Context, key string) clientv3.WatchChan {
	store, err := getStore()
	if err != nil {
		return nil
	}
	return store.WatchKeyWith(ctx, key)
}

// WatchKeys watches for changes on keys with the given prefix
func WatchKeys(keyPrefix string) clientv3.WatchChan {
	store, err := getStore()
	if err != nil {
		return nil
	}
	return store.WatchKeys(keyPrefix)
}

// WatchKeysWith watches for changes on keys with the given prefix with context
func WatchKeysWith(ctx context.Context, keyPrefix string) clientv3.WatchChan {
	store, err := getStore()
	if err != nil {
		return nil
	}
	return store.WatchKeysWith(ctx, keyPrefix)
}

// Close closes the store
func Close() error {
	store, err := getStore()
	if err != nil {
		return err
	}
	return store.Close()
}
