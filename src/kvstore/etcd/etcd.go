package etcd

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
)

// EtcdStore represents an etcd.
type EtcdStore struct {
	cli *clientv3.Client
	ctx context.Context
}

// Config holds the configuration for EtcdStore.
type Config struct {
	Endpoints   []string
	DialTimeout time.Duration
	Username    string
	Password    string
}

// NewEtcdStore creates a new instance of EtcdStore (singleton).
// It initializes the etcd client with the provided configuration and ensures only one instance is created.
func NewEtcdStore(ctx context.Context, config Config) (kvstore.Store, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: config.DialTimeout,
		Username:    config.Username,
		Password:    config.Password,
	})
	if err != nil {
		return nil, err
	}

	return &EtcdStore{cli: cli, ctx: ctx}, nil
}

// OpenSession creates a new etcd session.
// A session is needed for acquiring locks.
func (s *EtcdStore) NewSession(ctx context.Context) (*concurrency.Session, error) {
	return concurrency.NewSession(s.cli)
}

// NewLock acquires a lock on the given key and returns the mutex.
// It uses the provided session to ensure the lock's lifecycle is tied to the session.
func (s *EtcdStore) NewLock(ctx context.Context, session *concurrency.Session, lockKey string) (*concurrency.Mutex, error) {
	mutex := concurrency.NewMutex(session, lockKey)
	err := mutex.Lock(ctx)
	if err != nil {
		return nil, err
	}
	return mutex, nil
}

// Put stores a key-value pair in etcd.
func (s *EtcdStore) Put(key, value string) error {
	return s.PutWith(s.ctx, key, value)
}

// PutWith stores a key-value pair in etcd using the provided context.
func (s *EtcdStore) PutWith(ctx context.Context, key, value string) error {
	_, err := s.cli.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("failed to put key-value: %w", err)
	}
	return nil
}

// Get retrieves the value for a given key from etcd without using a context.
func (s *EtcdStore) Get(key string) (string, error) {
	return s.GetWith(s.ctx, key)
}

// GetWith retrieves the value for a given key from etcd using the provided context.
func (s *EtcdStore) GetWith(ctx context.Context, key string) (string, error) {
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("failed to get key: %w", err)
	}

	// Return the value of the first key-value pair found
	value := ""
	for _, kv := range resp.Kvs {
		value = string(kv.Value)
		return value, nil
	}

	// Return an empty string if no key-value pair is found
	return value, nil
}

// GetListWith retrieves multiple values for keys with the given keyPrefix from etcd.
func (s *EtcdStore) GetList(keyPrefix string) ([]string, error) {
	return s.GetListWith(s.ctx, keyPrefix)
}

// GetListWith retrieves multiple values for keys with the given keyPrefix from etcd using the provided context.
func (s *EtcdStore) GetListWith(ctx context.Context, keyPrefix string) ([]string, error) {
	// ascending by key as a default sort order
	optAscendByKey := clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)

	// Get all values with the given keyPrefix
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), optAscendByKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	values := []string{}
	for i, kv := range resp.Kvs {
		values[i] = string(kv.Value)
	}
	return values, nil
}

// GetKv retrieves a key-value pair from etcd without using a context.
func (s *EtcdStore) GetKv(key string) (kvstore.KeyValue, error) {
	return s.GetKvWith(s.ctx, key)
}

// GetKvWith retrieves a key-value pair from etcd using the provided context.
func (s *EtcdStore) GetKvWith(ctx context.Context, key string) (kvstore.KeyValue, error) {
	resp, err := s.cli.Get(ctx, key)
	if err != nil {
		return kvstore.KeyValue{}, fmt.Errorf("failed to get key: %w", err)
	}

	// Return the first key-value pair found
	keyValue := kvstore.KeyValue{}
	for _, kv := range resp.Kvs {
		keyValue = kvstore.KeyValue{Key: string(kv.Key), Value: string(kv.Value)}
		return keyValue, nil
	}

	// Return an empty key-value pair if no key-value pair is found
	return keyValue, nil
}

// GetKvList retrieves multiple key-value pairs with the given keyPrefix from etcd.
func (s *EtcdStore) GetKvList(keyPrefix string) ([]kvstore.KeyValue, error) {
	return s.GetKvListWith(s.ctx, keyPrefix)
}

// GetKvListWith retrieves multiple key-value pairs with the given keyPrefix from etcd using the provided context.
func (s *EtcdStore) GetKvListWith(ctx context.Context, keyPrefix string) ([]kvstore.KeyValue, error) {
	// ascending by key as a default sort order
	optAscendByKey := clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)

	// Get all key-value pairs with the given keyPrefix
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), optAscendByKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	kvs := []kvstore.KeyValue{}
	for _, kv := range resp.Kvs {
		kvs = append(kvs, kvstore.KeyValue{Key: string(kv.Key), Value: string(kv.Value)})
	}
	return kvs, nil
}

// GetSortedKvList retrieves multiple values for keys with the given keyPrefix, sortBy, and order from etcd.
func (s *EtcdStore) GetSortedKvList(keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	return s.GetSortedKvListWith(s.ctx, keyPrefix, sortBy, order)
}

// GetSortedKvListWith retrieves multiple values for keys with  the given keyPrefix, sortBy, and order from etcd using the provided context.
func (s *EtcdStore) GetSortedKvListWith(ctx context.Context, keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	sortOp := clientv3.WithSort(sortBy, order)
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), sortOp)
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	kvs := []kvstore.KeyValue{}
	for _, kv := range resp.Kvs {
		kvs = append(kvs, kvstore.KeyValue{Key: string(kv.Key), Value: string(kv.Value)})
	}
	return kvs, nil
}

// GetKvMap retrieves multiple key-value pairs with the given keyPrefix from etcd.
func (s *EtcdStore) GetKvMap(keyPrefix string) (kvstore.KeyValueMap, error) {
	return s.GetKvMapWith(s.ctx, keyPrefix)
}

// GetKvMapWith retrieves multiple key-value pairs with the given keyPrefix from etcd using the provided context.
func (s *EtcdStore) GetKvMapWith(ctx context.Context, keyPrefix string) (kvstore.KeyValueMap, error) {
	// ascending by key as a default sort order
	optAscendByKey := clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend)

	// Get all key-value pairs with the given keyPrefix
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), optAscendByKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	kvs := kvstore.KeyValueMap{}
	for _, kv := range resp.Kvs {
		kvs[string(kv.Key)] = string(kv.Value)
	}
	return kvs, nil
}

// Delete removes a key-value pair from etcd without using a context.
func (s *EtcdStore) Delete(key string) error {
	return s.DeleteWith(s.ctx, key)
}

// DeleteWith removes a key-value pair from etcd using the provided context.
func (s *EtcdStore) DeleteWith(ctx context.Context, key string) error {
	_, err := s.cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// WatchKey watches for changes on the given key.
func (s *EtcdStore) WatchKey(key string) clientv3.WatchChan {
	return s.WatchKeyWith(s.ctx, key)
}

// WatchKeyWith watches for changes on the given key using the provided context.
func (s *EtcdStore) WatchKeyWith(ctx context.Context, key string) clientv3.WatchChan {
	return s.cli.Watch(ctx, key)
}

// WatchKeys watches for changes on keys with the given keyPrefix.
func (s *EtcdStore) WatchKeys(keyPrefix string) clientv3.WatchChan {
	return s.WatchKeysWith(s.ctx, keyPrefix)
}

// WatchKeysWith watches for changes on keys with the given keyPrefix using the provided context.
func (s *EtcdStore) WatchKeysWith(ctx context.Context, keyPrefix string) clientv3.WatchChan {
	return s.cli.Watch(ctx, keyPrefix, clientv3.WithPrefix())
}

// Close closes the etcd client.
// This is necessary to release resources associated with the client.
func (s *EtcdStore) Close() error {
	return s.cli.Close()
}

// // CloseSession closes the given etcd session.
// // It's important to close sessions to release resources.
// func (s *EtcdStore) CloseSession(session *concurrency.Session) error {
// 	return session.Close()
// }

// // Unlock releases the given lock.
// // It is important to release the lock to allow other clients to acquire it.
// func (s *EtcdStore) Unlock(ctx context.Context, mutex *concurrency.Mutex) error {
// 	return mutex.Unlock(ctx)
// }
