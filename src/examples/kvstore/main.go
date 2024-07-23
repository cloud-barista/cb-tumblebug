package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
)

func main() {
	// EtcdStore configuration
	config := kvstore.Config{
		Endpoints:   []string{"localhost:2379"}, // Replace with your etcd server endpoints
		DialTimeout: 5 * time.Second,
	}

	// Create EtcdStore instance (singleton)
	ctx := context.Background()
	etcd, err := kvstore.NewEtcd(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create EtcdStore: %v", err)
	}
	defer etcd.Close()

	ctx2 := context.Background() // Create context for etcd operations

	// Basic CRUD operations test
	fmt.Println("\n## Basic CRUD operations test")
	ExampleBasicCRUDTest(ctx2, etcd)

	// Race condition test
	fmt.Println("\n## ExampleRaceConditionTest")
	ExampleRaceConditionTest(ctx2, etcd)

	// FilterKVsBy example
	fmt.Println("\n## FilterKVsBy example")
	ExampleFilterKVsBy()

	// ExtractIDsFromKey example
	fmt.Println("\n## ExtractIDsFromKey example")
	ExampleExtractIDsFromKey()

	// ContainsIDs example
	fmt.Println("\n## ContainsIDs example")
	ExampleContainsIDs()

	// [May not needed] BuildKey example
	// fmt.Println("\n## BuildKey example")
	// ExampleBuildKey()

	// Watch operations example
	fmt.Println("\n## Watch operations example")

	var wg sync.WaitGroup
	ctx3, cancel := context.WithCancel(context.Background())
	defer cancel()
	// goroutine to watch a single key
	wg.Add(1)
	go watchSingleKey(ctx3, &wg, etcd)

	// goroutine to watch keys
	wg.Add(1)
	go watchMultipleKeys(ctx3, &wg, etcd)

	// goroutine to update values
	wg.Add(1)
	go changeValues(ctx3, &wg, etcd)

	// Wait for 10 seconds and then cancel the context to stop the goroutines
	time.Sleep(10 * time.Second)
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	fmt.Println("\nAll operations completed successfully!")
}

func ExampleBasicCRUDTest(ctx context.Context, etcd kvstore.Store) {
	key := "test_key"
	value := "Hello, Etcd!"

	// Put (Store) a key-value pair
	err := etcd.PutWith(ctx, key, value)
	if err != nil {
		log.Fatalf("Failed to put key-value: %v", err)
	}
	fmt.Printf("Successfully put key '%s' with value '%s'\n", key, value)

	// Get (Retrieve) the value
	retrievedValue, err := etcd.GetWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get value: %v", err)
	}
	fmt.Printf("Retrieved value for key '%s': %s\n", key, retrievedValue)

	// Update the value
	newValue := "Updated Etcd Value"
	err = etcd.PutWith(ctx, key, newValue)
	if err != nil {
		log.Fatalf("Failed to update value: %v", err)
	}
	fmt.Printf("Successfully updated key '%s' with new value '%s'\n", key, newValue)

	// Get (Retrieve) the updated value
	retrievedValue, err = etcd.GetWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get updated value: %v", err)
	}
	fmt.Printf("Retrieved updated value for key '%s': %s\n", key, retrievedValue)

	// Delete the key-value pair
	err = etcd.DeleteWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to delete key: %v", err)
	}
	fmt.Printf("Successfully deleted key '%s'\n", key)

	// Verify deletion
	_, err = etcd.GetWith(ctx, key)
	if err != nil {
		fmt.Printf("As expected, failed to get deleted key '%s': %v\n", key, err)
	} else {
		log.Fatalf("Unexpectedly succeeded in getting deleted key '%s'", key)
	}
}

func ExampleRaceConditionTest(ctx context.Context, etcd kvstore.Store) {
	fmt.Println("Starting race condition test...")

	key := "race_test_key"
	iterations := 100
	goroutines := 5

	// Initialize the key with 0
	err := etcd.PutWith(ctx, key, "0")
	if err != nil {
		log.Fatalf("Failed to initialize key: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Start goroutines
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			// Create a persistent session
			session, err := etcd.NewSession(ctx)
			if err != nil {
				log.Fatalf("Failed to create etcd session: %v", err)
			}
			defer session.Close()

			// Get Lock
			lockKey := key
			// lock, err := etcd.NewLock(ctx, session, lockKey)
			lock, err := etcd.NewLock(ctx, session, lockKey)
			if err != nil {
				log.Fatalf("Failed to get lock: %v", err)
			}

			for j := 0; j < iterations; j++ {

				err = lock.Lock(ctx)
				if err != nil {
					log.Printf("Failed to acquire lock: %v", err)
					continue
				}

				// Get current value, increment, and put new value within the lock
				value, err := etcd.GetWith(ctx, key)
				if err != nil {
					log.Printf("Failed to get value: %v", err)
					// Unlock
					err = lock.Unlock(ctx)
					if err != nil {
						log.Printf("Failed to release lock: %v", err)
					}
					continue
				}

				intValue, _ := strconv.Atoi(value)
				newValue := fmt.Sprintf("%d", intValue+1)

				err = etcd.PutWith(ctx, key, newValue)
				if err != nil {
					log.Printf("Failed to put value: %v", err)
					// Unlock
					err = lock.Unlock(ctx)
					if err != nil {
						log.Printf("Failed to release lock: %v", err)
					}
					continue
				}
				log.Printf("Put value: %s", newValue)

				// Unlock
				err = lock.Unlock(ctx)
				if err != nil {
					log.Printf("Failed to release lock: %v", err)
					continue
				}
			}
		}()
	}

	wg.Wait()

	// Verify the final value
	finalValue, err := etcd.GetWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get final value: %v", err)
	}

	expectedValue := goroutines * iterations
	actualValue, _ := strconv.Atoi(finalValue)
	if actualValue != expectedValue {
		log.Fatalf("Race condition detected. Expected %d, but got %d", expectedValue, actualValue)
	}

	fmt.Printf("Race condition test finished. Final value: %s\n", finalValue)

	// Clean up
	etcd.DeleteWith(ctx, key)
}

// ExampleFilterKVsBy demonstrates the usage of the FilterKVsBy function
// with various key values and different levels of depth.
func ExampleFilterKVsBy() {
	kvs := kvstore.KeyValueMap{
		"/ns/ns01/mcis/mcis02":           "value1",
		"/ns/ns01/mcis/mcis03":           "value2",
		"/ns/ns04/mcis/mcis02":           "value3",
		"/ns/ns01":                       "value4",
		"/ns/ns04/mcis/mcis05/vpc/vpc01": "value5",
		"/ns/ns01/mcis/mcis07":           "value6",
	}

	// Print all key-value pairs
	fmt.Println("All key-value pairs:")
	for key, value := range kvs {
		fmt.Println(key, value)
	}

	// Case 1: Filter by ns=ns01 and mcis=id2
	prefixkey1 := "/ns/ns01/mcis"
	filteredKVs1 := kvutil.FilterKvMapBy(kvs, prefixkey1)
	fmt.Println("Filtered by '/ns/ns01/mcis', Ouput 'ns/ns01/mcis/{mcisId}': ")
	for key, value := range filteredKVs1 {
		fmt.Println(key, value)
	}
	// Output: /ns/ns01/mcis/mcis02 value1
	// Output: /ns/ns01/mcis/mcis03 value2
	// Output: /ns/ns01/mcis/mcis07 value6

	// Case 2: Filter by ns=ns01
	prefixkey2 := "/ns"
	filteredKVs2 := kvutil.FilterKvMapBy(kvs, prefixkey2)
	fmt.Println("Filtered by '/ns', Ouput 'ns/{nsId}'")
	for key, value := range filteredKVs2 {
		fmt.Println(key, value)
	}
	// Output: /ns/ns01 value4

	// Case 3: Filter by ns=ns04, mcis=mcis05, and vpc=vpc01
	prefixkey3 := "/ns/ns04/mcis/mcis05/vpc"
	filteredKVs3 := kvutil.FilterKvMapBy(kvs, prefixkey3)
	fmt.Println("Filtered by '/ns/ns04/mcis/mcis05/vpc', Ouput '/ns/ns04/mcis/mcis05/vpc/{vpcId}'")
	for key, value := range filteredKVs3 {
		fmt.Println(key, value)
	}
	// Output: /ns/ns04/mcis/mcis05/vpc/vpc01 value5
}

func ExampleExtractIDsFromKey() {

	key := "/ns/ns01/mcis/mcis02/vpc/vpc03"

	ids, err := kvutil.ExtractIDsFromKey(key, "ns", "mcis", "vpc")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Key: ", key)
	fmt.Println(ids)

	key2 := "/ns/ns01/mcis/mcis02/SOMETHINGADDED/vpc/vpc03"

	ids, err = kvutil.ExtractIDsFromKey(key2, "ns", "mcis", "vpc")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Key: ", key2)
	fmt.Println(ids)
	// Output: [ns01 id2 id3]
}

func ExampleContainsIDs() {

	ids := map[string]string{
		"ns":   "ns01",
		"mcis": "mcis02",
	}

	key := "/ns/ns01/mcis/mcis02/vpc/vpc03"
	contains := kvutil.ContainsIDs(key, ids)
	fmt.Println("key: ", key)
	fmt.Println("ids: ", ids)
	fmt.Println("result: ", contains)

	key2 := "/ns/ns01/mcis/mcis02/SOMETHINGADDED/vpc/vpc03"
	contains = kvutil.ContainsIDs(key2, ids)
	fmt.Println("key: ", key2)
	fmt.Println("ids: ", ids)
	fmt.Println("result: ", contains)
	// Output: true
}

// [May not needed]
// func ExampleBuildKey() {
// 	ids := map[string]string{
// 		"ns":   "ns01",
// 		"mcis": "mcis02",
// 		"vpc":  "vpc03",
// 	}

// 	key := kvstore.BuildKeyBy(ids)
// 	fmt.Println(key)
// 	// Output: /ns/ns01/mcis/mcis02/vpc/vpc03
// }

func watchSingleKey(ctx context.Context, wg *sync.WaitGroup, etcd kvstore.Store) {
	defer wg.Done()
	watchChan := etcd.WatchKeyWith(ctx, "mykey")
	for {
		select {
		case resp, ok := <-watchChan:
			if !ok {
				fmt.Println("Watch channel closed")
				return
			}
			for _, ev := range resp.Events {
				fmt.Printf("(Single key watch) Type: %s Key: %s Value: %s\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		case <-ctx.Done():
			fmt.Println("Single key watch cancelled")
			return
		}
	}
}

func watchMultipleKeys(ctx context.Context, wg *sync.WaitGroup, etcd kvstore.Store) {
	defer wg.Done()
	watchChan := etcd.WatchKeysWith(ctx, "myprefix")
	for {
		select {
		case resp, ok := <-watchChan:
			if !ok {
				fmt.Println("Watch channel closed")
				return
			}
			for _, ev := range resp.Events {
				fmt.Printf("(Multiple keys watch) Type: %s Key: %s Value: %s\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			}
		case <-ctx.Done():
			fmt.Println("Multiple keys watch cancelled")
			return
		}
	}
}

func changeValues(ctx context.Context, wg *sync.WaitGroup, etcd kvstore.Store) {
	defer wg.Done()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("Change values cancelled")
			return
		default:
			// Update value with a single key
			err := etcd.PutWith(ctx, "mykey", fmt.Sprintf("value%d", i))
			if err != nil {
				log.Printf("Error putting mykey: %v", err)
			}

			// Update values with multiple keys
			err = etcd.PutWith(ctx, fmt.Sprintf("myprefix/key%d", i), fmt.Sprintf("prefixvalue%d", i))
			if err != nil {
				log.Printf("Error putting myprefix/key%d: %v", i, err)
			}

			time.Sleep(1 * time.Second)
		}
	}
}
