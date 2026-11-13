package etcd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
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
func (s *EtcdStore) Get(key string) (string, bool, error) {
	return s.GetWith(s.ctx, key)
}

// GetWith retrieves the value for a given key from etcd using the provided context.
func (s *EtcdStore) GetWith(ctx context.Context, key string) (string, bool, error) {
	resp, err := s.cli.Get(ctx, key, clientv3.WithSerializable())
	if err != nil {
		return "", false, fmt.Errorf("failed to get key: %w", err)
	}

	if len(resp.Kvs) > 0 {
		kv := resp.Kvs[0]
		return string(kv.Value), true, nil
	}

	// Return an empty string if no key-value pair is found
	return "", false, nil
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
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), optAscendByKey, clientv3.WithSerializable())
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	values := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		values = append(values, string(kv.Value))
	}
	return values, nil
}

// GetKv retrieves a key-value pair from etcd without using a context.
func (s *EtcdStore) GetKv(key string) (kvstore.KeyValue, bool, error) {
	return s.GetKvWith(s.ctx, key)
}

// GetKvWith retrieves a key-value pair from etcd using the provided context.
func (s *EtcdStore) GetKvWith(ctx context.Context, key string) (kvstore.KeyValue, bool, error) {
	resp, err := s.cli.Get(ctx, key, clientv3.WithSerializable())
	if err != nil {
		return kvstore.KeyValue{}, false, fmt.Errorf("failed to get key: %w", err)
	}

	// Return the first key-value pair found
	if len(resp.Kvs) > 0 {
		kv := resp.Kvs[0]
		keyValue := kvstore.KeyValue{Key: string(kv.Key), Value: string(kv.Value)}
		return keyValue, true, nil
	}

	// Return an empty key-value pair if no key-value pair is found
	return kvstore.KeyValue{}, false, nil
}

// GetKvList retrieves multiple key-value pairs with the given keyPrefix from etcd.
func (s *EtcdStore) GetKvList(keyPrefix string) ([]kvstore.KeyValue, error) {
	return s.GetKvListWith(s.ctx, keyPrefix)
}

// GetKvListWith retrieves multiple key-value pairs with the given keyPrefix from etcd using the provided context,
// ascending by key.
func (s *EtcdStore) GetKvListWith(ctx context.Context, keyPrefix string) ([]kvstore.KeyValue, error) {
	raw, err := s.rangePaged(ctx, keyPrefix, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}
	kvs := make([]kvstore.KeyValue, 0, len(raw))
	for _, kv := range raw {
		kvs = append(kvs, kvstore.KeyValue{Key: string(kv.Key), Value: string(kv.Value)})
	}
	return kvs, nil
}

// rangePageSize bounds one Get so a prefix holding tens of thousands of multi-KB values (an Infra's Node
// records or init results) arrives as a series of modest responses instead of one of tens of MB.
const rangePageSize = 1000

// rangePaged reads every key under keyPrefix in pages pinned to one revision, so the pages form a consistent
// snapshot. keysOnly skips the values.
func (s *EtcdStore) rangePaged(ctx context.Context, keyPrefix string, keysOnly bool) ([]*mvccpb.KeyValue, error) {
	start, end := keyPrefix, clientv3.GetPrefixRangeEnd(keyPrefix)
	if keyPrefix == "" {
		start, end = "\x00", "\x00"
	}
	key, rev := start, int64(0)
	var out []*mvccpb.KeyValue
	for {
		opts := []clientv3.OpOption{clientv3.WithRange(end), clientv3.WithLimit(rangePageSize),
			clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend), clientv3.WithSerializable()}
		if keysOnly {
			opts = append(opts, clientv3.WithKeysOnly())
		}
		if rev > 0 {
			opts = append(opts, clientv3.WithRev(rev))
		}
		resp, err := s.cli.Get(ctx, key, opts...)
		if err != nil {
			if rev > 0 && errors.Is(err, rpctypes.ErrCompacted) {
				// the pinned revision was compacted mid-read: start over on the current one
				key, rev, out = start, 0, nil
				continue
			}
			return nil, err
		}
		if rev == 0 {
			rev = resp.Header.Revision
		}
		out = append(out, resp.Kvs...)
		if !resp.More || len(resp.Kvs) == 0 {
			return out, nil
		}
		key = string(resp.Kvs[len(resp.Kvs)-1].Key) + "\x00"
	}
}

// GetKeyList retrieves only keys (no values) with the given keyPrefix from etcd.
func (s *EtcdStore) GetKeyList(keyPrefix string) ([]string, error) {
	return s.GetKeyListWith(s.ctx, keyPrefix)
}

// GetKeyListWith retrieves only keys with the given keyPrefix from etcd using the provided context.
func (s *EtcdStore) GetKeyListWith(ctx context.Context, keyPrefix string) ([]string, error) {
	raw, err := s.rangePaged(ctx, keyPrefix, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get key list with keyPrefix: %w", err)
	}
	keys := make([]string, 0, len(raw))
	for _, kv := range raw {
		keys = append(keys, string(kv.Key))
	}
	return keys, nil
}

// childPageSize is how many keys one step of GetChildListWith reads. A prefix with fewer keys is listed in a
// single request; a child with more keys below it costs one page before the rest of its subtree is skipped.
const childPageSize = 500

// GetChildList returns the names directly under keyPrefix.
func (s *EtcdStore) GetChildList(keyPrefix string) ([]kvstore.ChildKey, error) {
	return s.GetChildListWith(s.ctx, keyPrefix)
}

// GetChildListWith returns the names directly under keyPrefix. It reads keys in pages and, when a page ends
// inside a child's subtree, continues right after that subtree instead of reading through it. etcd has no
// "list one level" request, and scanning a whole subtree to find its top-level names costs as much as the
// subtree is large (tens of MB for an Infra with thousands of Nodes).
func (s *EtcdStore) GetChildListWith(ctx context.Context, keyPrefix string) ([]kvstore.ChildKey, error) {
	end := clientv3.GetPrefixRangeEnd(keyPrefix)
	cur, rev := keyPrefix, int64(0)
	collector := kvstore.NewChildCollector(keyPrefix)
	for {
		opts := []clientv3.OpOption{clientv3.WithRange(end), clientv3.WithLimit(childPageSize),
			clientv3.WithKeysOnly(), clientv3.WithSerializable()}
		if rev > 0 {
			opts = append(opts, clientv3.WithRev(rev))
		}
		resp, err := s.cli.Get(ctx, cur, opts...)
		if err != nil {
			if rev > 0 && errors.Is(err, rpctypes.ErrCompacted) {
				// the pinned revision was compacted mid-read: start over on the current one
				cur, rev, collector = keyPrefix, 0, kvstore.NewChildCollector(keyPrefix)
				continue
			}
			return nil, fmt.Errorf("failed to list children of keyPrefix: %w", err)
		}
		if rev == 0 {
			rev = resp.Header.Revision
		}
		name, below := "", false
		for _, kv := range resp.Kvs {
			name, below = collector.Add(string(kv.Key))
		}
		if !resp.More || len(resp.Kvs) == 0 {
			return collector.Children(), nil
		}
		if below {
			cur = keyPrefix + name + "0" // "/" + 1: the first key after this child's subtree
		} else {
			cur = string(resp.Kvs[len(resp.Kvs)-1].Key) + "\x00"
		}
	}
}

// GetSortedKvList retrieves multiple values for keys with the given keyPrefix, sortBy, and order from etcd.
func (s *EtcdStore) GetSortedKvList(keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	return s.GetSortedKvListWith(s.ctx, keyPrefix, sortBy, order)
}

// GetSortedKvListWith retrieves multiple values for keys with  the given keyPrefix, sortBy, and order from etcd using the provided context.
func (s *EtcdStore) GetSortedKvListWith(ctx context.Context, keyPrefix string, sortBy clientv3.SortTarget, order clientv3.SortOrder) ([]kvstore.KeyValue, error) {
	sortOp := clientv3.WithSort(sortBy, order)
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), sortOp, clientv3.WithSerializable())
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	kvs := make([]kvstore.KeyValue, 0, len(resp.Kvs))
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
	resp, err := s.cli.Get(ctx, keyPrefix, clientv3.WithPrefix(), optAscendByKey, clientv3.WithSerializable())
	if err != nil {
		return nil, fmt.Errorf("failed to get list with keyPrefix: %w", err)
	}

	kvs := make(kvstore.KeyValueMap, len(resp.Kvs))
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

// DeleteWithPrefix removes all key-value pairs with the given prefix in a single etcd request.
func (s *EtcdStore) DeleteWithPrefix(keyPrefix string) error {
	return s.DeleteWithPrefixWith(s.ctx, keyPrefix)
}

// DeleteWithPrefixWith removes all key-value pairs with the given prefix in a single etcd request using the provided context.
func (s *EtcdStore) DeleteWithPrefixWith(ctx context.Context, keyPrefix string) error {
	_, err := s.cli.Delete(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("failed to delete keys with prefix %q: %w", keyPrefix, err)
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

// Compact discards all MVCC history up to the current revision, marking the
// freed space as reusable. This does not shrink the on-disk database file —
// call Defragment afterward to actually reclaim disk space. Without periodic
// compaction, ordinary writes accumulate revision history forever and can
// eventually exhaust etcd's default 2GiB backend quota.
func (s *EtcdStore) Compact(ctx context.Context) error {
	endpoints := s.cli.Endpoints()
	if len(endpoints) == 0 {
		return fmt.Errorf("no etcd endpoints configured")
	}
	status, err := s.cli.Status(ctx, endpoints[0])
	if err != nil {
		return fmt.Errorf("failed to get etcd status: %w", err)
	}
	if _, err := s.cli.Compact(ctx, status.Header.Revision); err != nil {
		if errors.Is(err, rpctypes.ErrCompacted) {
			// Nothing has been written since the last compaction reached this
			// same revision — not an error, just nothing new to reclaim.
			return nil
		}
		return fmt.Errorf("failed to compact etcd to revision %d: %w", status.Header.Revision, err)
	}
	return nil
}

// Defragment rewrites the backend database file on every configured
// endpoint to reclaim space freed by Compact. It is a blocking, I/O-heavy
// operation on the server side; endpoints are defragmented one at a time
// (never concurrently), which is required to avoid losing quorum in a
// multi-member cluster.
func (s *EtcdStore) Defragment(ctx context.Context) error {
	for _, ep := range s.cli.Endpoints() {
		if _, err := s.cli.Defragment(ctx, ep); err != nil {
			return fmt.Errorf("failed to defragment etcd endpoint %s: %w", ep, err)
		}
	}
	return nil
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
