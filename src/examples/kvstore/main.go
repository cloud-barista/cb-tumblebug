package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/etcd"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvutil"
)

func main() {
	// EtcdStore configuration
	config := etcd.Config{
		Endpoints:   []string{"localhost:2379"}, // Replace with your etcd server endpoints
		DialTimeout: 5 * time.Second,
		Username:    "default",
		Password:    "default",
	}

	// Create EtcdStore instance (singleton)
	ctx := context.Background()
	etcd, err := etcd.NewEtcdStore(ctx, config)
	if err != nil {
		log.Fatalf("Failed to create EtcdStore: %v", err)
	}
	defer etcd.Close()

	// Initialize global Store with EtcdStore
	err = kvstore.InitializeStore(etcd)
	if err != nil {
		log.Fatalf("Failed to initialize global Store: %v", err)
	}

	ctx2 := context.Background() // Create context for etcd operations

	// Basic CRUD operations test
	fmt.Println("\n## Basic CRUD operations test")
	ExampleBasicCRUDTest(ctx2)

	// Race condition test
	fmt.Println("\n## ExampleRaceConditionTest")
	ExampleRaceConditionTest(ctx2)

	// FilterKvMapBy example
	fmt.Println("\n## FilterKvListBy example")
	ExampleFilterKvListBy()

	// FilterKvMapBy example
	fmt.Println("\n## FilterKvMapBy example")
	ExampleFilterKvMapBy()

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
	go watchSingleKey(ctx3, &wg)

	// goroutine to watch keys
	wg.Add(1)
	go watchMultipleKeys(ctx3, &wg)

	// goroutine to update values
	wg.Add(1)
	go changeValues(ctx3, &wg)

	// Wait for 10 seconds and then cancel the context to stop the goroutines
	time.Sleep(10 * time.Second)
	cancel()

	// Wait for all goroutines to finish
	wg.Wait()

	fmt.Println("\nAll operations completed successfully!")

	fmt.Println("\nAfter 10 seconds, delete some example keys and values")
	time.Sleep(10 * time.Second)

	kvstore.Delete("/mykey")
	for i := 0; i < 10; i++ {
		kvstore.Delete("/myprefixkey/key" + strconv.Itoa(i))
	}

	time.Sleep(5 * time.Second)
}

func ExampleBasicCRUDTest(ctx context.Context) {
	key := "/test_key"
	value := "Hello, Etcd!"

	// Put (Store) a key-value pair
	err := kvstore.PutWith(ctx, key, value)
	if err != nil {
		log.Fatalf("Failed to put key-value: %v", err)
	}
	fmt.Printf("Successfully put key '%s' with value '%s'\n", key, value)

	// Get (Retrieve) the value
	retrievedValue, _, err := kvstore.GetWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get value: %v", err)
	}
	fmt.Printf("Retrieved value for key '%s': %s\n", key, retrievedValue)

	// Update the value
	newValue := "Updated Etcd Value"
	err = kvstore.PutWith(ctx, key, newValue)
	if err != nil {
		log.Fatalf("Failed to update value: %v", err)
	}
	fmt.Printf("Successfully updated key '%s' with new value '%s'\n", key, newValue)

	// Get (Retrieve) the updated value
	retrievedValue, _, err = kvstore.GetWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to get updated value: %v", err)
	}
	fmt.Printf("Retrieved updated value for key '%s': %s\n", key, retrievedValue)

	// Delete the key-value pair
	err = kvstore.DeleteWith(ctx, key)
	if err != nil {
		log.Fatalf("Failed to delete key: %v", err)
	}
	fmt.Printf("Successfully deleted key '%s'\n", key)

	// Verify deletion
	_, _, err = kvstore.GetWith(ctx, key)
	if err != nil {
		fmt.Printf("As expected, failed to get deleted key '%s': %v\n", key, err)
	} else {
		log.Fatalf("Unexpectedly succeeded in getting deleted key '%s'", key)
	}
}

func ExampleRaceConditionTest(ctx context.Context) {
	fmt.Println("Starting race condition test...")

	key := "/race_test_key"
	iterations := 100
	goroutines := 5

	// Initialize the key with 0
	err := kvstore.PutWith(ctx, key, "0")
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
			session, err := kvstore.NewSession(ctx)
			if err != nil {
				log.Fatalf("Failed to create etcd session: %v", err)
			}
			defer session.Close()

			// Get Lock
			lockKey := key
			// lock, err := kvstore.NewLock(ctx, session, lockKey)
			lock, err := kvstore.NewLock(ctx, session, lockKey)
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
				value, _, err := kvstore.GetWith(ctx, key)
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

				err = kvstore.PutWith(ctx, key, newValue)
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
	finalValue, _, err := kvstore.GetWith(ctx, key)
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
	kvstore.DeleteWith(ctx, key)
}

func ExampleFilterKvListBy() {
	kvs := []kvstore.KeyValue{
		{Key: "/ns/default/mci/mci02", Value: "value1"},
		{Key: "/ns/default/mci/mci02/", Value: "value1"},
		{Key: "/ns/default/mci/mci03", Value: "value2"},
		{Key: "/ns/default/mci/mci03/", Value: "value2"},
		{Key: "/ns/ns04/mci/mci02", Value: "value3"},
		{Key: "/ns/ns04/mci/mci02/", Value: "value3"},
		{Key: "/ns/default", Value: "value4"},
		{Key: "/ns/default/", Value: "value4"},
		{Key: "/ns/ns04/mci/mci05/vpc/vpc01", Value: "value5"},
		{Key: "/ns/ns04/mci/mci05/vpc/vpc01/", Value: "value5"},
		{Key: "/ns/default/mci/mci07", Value: "value6"},
		{Key: "/ns/default/mci/mci07/", Value: "value6"},
	}

	// Print all key-value pairs
	fmt.Println("\nAll key-value pairs:")
	for _, kv := range kvs {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 1-1: Filter by ns=default and mci=id2
	prefixkey11 := "/ns/default/mci"
	filteredKVs11 := kvutil.FilterKvListBy(kvs, prefixkey11, 1)
	fmt.Println("\nFiltered by '/ns/default/mci', Output 'ns/default/mci/{mciId}': ")
	for _, kv := range filteredKVs11 {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 1-2: Filter by ns=default and mci=id2
	prefixkey12 := "/ns/default/mci/"
	filteredKVs12 := kvutil.FilterKvListBy(kvs, prefixkey12, 1)
	fmt.Println("\nFiltered by '/ns/default/mci/', Output 'ns/default/mci/{mciId}': ")
	for _, kv := range filteredKVs12 {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 2-1: Filter by ns=default
	prefixkey21 := "/ns"
	filteredKVs21 := kvutil.FilterKvListBy(kvs, prefixkey21, 1)
	fmt.Println("\nFiltered by '/ns', Output 'ns/{nsId}'")
	for _, kv := range filteredKVs21 {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 2-2: Filter by ns=default
	prefixkey22 := "/ns/"
	filteredKVs22 := kvutil.FilterKvListBy(kvs, prefixkey22, 1)
	fmt.Println("\nFiltered by '/ns/', Output 'ns/{nsId}'")
	for _, kv := range filteredKVs22 {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 3-1: Filter by ns=ns04, mci=mci05, and vpc=vpc01
	prefixkey31 := "/ns/ns04/mci/mci05/vpc"
	filteredKVs31 := kvutil.FilterKvListBy(kvs, prefixkey31, 1)
	fmt.Println("\nFiltered by '/ns/ns04/mci/mci05/vpc', Output '/ns/ns04/mci/mci05/vpc/{vpcId}'")
	for _, kv := range filteredKVs31 {
		fmt.Println(kv.Key, kv.Value)
	}

	// Case 3-2: Filter by ns=ns04, mci=mci05, and vpc=vpc01
	prefixkey32 := "/ns/ns04/mci/mci05/vpc/"
	filteredKVs32 := kvutil.FilterKvListBy(kvs, prefixkey32, 1)
	fmt.Println("\nFiltered by '/ns/ns04/mci/mci05/vpc', Output '/ns/ns04/mci/mci05/vpc/{vpcId}'")
	for _, kv := range filteredKVs32 {
		fmt.Println(kv.Key, kv.Value)
	}
}

// ExampleFilterKvMapBy demonstrates the usage of the FilterKVsBy function
// with various key values and different levels of depth.
func ExampleFilterKvMapBy() {
	kvs := kvstore.KeyValueMap{
		"/ns/default/mci/mci02":        "value1",
		"/ns/default/mci/mci03":        "value2",
		"/ns/ns04/mci/mci02":           "value3",
		"/ns/default":                  "value4",
		"/ns/ns04/mci/mci05/vpc/vpc01": "value5",
		"/ns/default/mci/mci07":        "value6",
	}

	// Print all key-value pairs
	fmt.Println("\nAll key-value pairs:")
	for key, value := range kvs {
		fmt.Println(key, value)
	}

	// Case 1: Filter by ns=default and mci=id2
	prefixkey1 := "/ns/default/mci"
	filteredKVs1 := kvutil.FilterKvMapBy(kvs, prefixkey1, 1)
	fmt.Println("\nFiltered by '/ns/default/mci', Output 'ns/default/mci/{mciId}': ")
	for key, value := range filteredKVs1 {
		fmt.Println(key, value)
	}
	// Output: /ns/default/mci/mci02 value1
	// Output: /ns/default/mci/mci03 value2
	// Output: /ns/default/mci/mci07 value6

	// Case 2: Filter by ns=default
	prefixkey2 := "/ns"
	filteredKVs2 := kvutil.FilterKvMapBy(kvs, prefixkey2, 1)
	fmt.Println("\nFiltered by '/ns', Output 'ns/{nsId}'")
	for key, value := range filteredKVs2 {
		fmt.Println(key, value)
	}
	// Output: /ns/default value4

	// Case 3: Filter by ns=ns04, mci=mci05, and vpc=vpc01
	prefixkey3 := "/ns/ns04/mci/mci05/vpc"
	filteredKVs3 := kvutil.FilterKvMapBy(kvs, prefixkey3, 1)
	fmt.Println("\nFiltered by '/ns/ns04/mci/mci05/vpc', Output '/ns/ns04/mci/mci05/vpc/{vpcId}'")
	for key, value := range filteredKVs3 {
		fmt.Println(key, value)
	}
	// Output: /ns/ns04/mci/mci05/vpc/vpc01 value5
}

func ExampleExtractIDsFromKey() {

	key := "/ns/default/mci/mci02/vpc/vpc03"

	ids, err := kvutil.ExtractIDsFromKey(key, "ns", "mci", "vpc")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Key: ", key)
	fmt.Println(ids)

	key2 := "/ns/default/mci/mci02/SOMETHINGADDED/vpc/vpc03"

	ids, err = kvutil.ExtractIDsFromKey(key2, "ns", "mci", "vpc")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Key: ", key2)
	fmt.Println(ids)
	// Output: [default id2 id3]
}

func ExampleContainsIDs() {

	ids := map[string]string{
		"ns":  "default",
		"mci": "mci02",
	}

	key := "/ns/default/mci/mci02/vpc/vpc03"
	contains := kvutil.ContainsIDs(key, ids)
	fmt.Println("key: ", key)
	fmt.Println("ids: ", ids)
	fmt.Println("result: ", contains)

	key2 := "/ns/default/mci/mci02/SOMETHINGADDED/vpc/vpc03"
	contains = kvutil.ContainsIDs(key2, ids)
	fmt.Println("key: ", key2)
	fmt.Println("ids: ", ids)
	fmt.Println("result: ", contains)
	// Output: true
}

// [May not needed]
// func ExampleBuildKey() {
// 	ids := map[string]string{
// 		"ns":   "default",
// 		"mci": "mci02",
// 		"vpc":  "vpc03",
// 	}

// 	key := kvstore.BuildKeyBy(ids)
// 	fmt.Println(key)
// 	// Output: /ns/default/mci/mci02/vpc/vpc03
// }

func watchSingleKey(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	watchChan := kvstore.WatchKeyWith(ctx, "/mykey")
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

func watchMultipleKeys(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	watchChan := kvstore.WatchKeysWith(ctx, "/myprefixkey")
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

func changeValues(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("Change values cancelled")
			return
		default:
			// Update value with a single key
			err := kvstore.PutWith(ctx, "/mykey", fmt.Sprintf("value%d", i))
			if err != nil {
				log.Printf("Error putting /mykey: %v", err)
			}

			// Update values with multiple keys
			err = kvstore.PutWith(ctx, fmt.Sprintf("/myprefixkey/key%d", i), fmt.Sprintf("prefixvalue%d", i))
			if err != nil {
				log.Printf("Error putting /myprefixkey/key%d: %v", i, err)
			}

			time.Sleep(1 * time.Second)
		}
	}
}
