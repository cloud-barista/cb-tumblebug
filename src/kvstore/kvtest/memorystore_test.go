package kvtest

import (
	"context"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
)

func TestMemoryStore(t *testing.T) {
	mem := NewMemoryStore()
	cleanup := kvstore.SetTestStore(mem)
	defer cleanup()

	ctx := context.Background()

	// 1. Put & Get
	err := kvstore.PutWith(ctx, "/test/key1", "val1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, found, err := kvstore.GetWith(ctx, "/test/key1")
	if err != nil || !found || val != "val1" {
		t.Fatalf("Get failed: got (%q, %v, %v), want ('val1', true, nil)", val, found, err)
	}

	// 2. Put second key
	err = kvstore.PutWith(ctx, "/test/key2", "val2")
	if err != nil {
		t.Fatalf("Put key2 failed: %v", err)
	}

	// 3. GetList
	list, err := kvstore.GetListWith(ctx, "/test/")
	if err != nil || len(list) != 2 {
		t.Fatalf("GetList failed: got %v, err: %v", list, err)
	}

	// 4. GetKvList
	kvList, err := kvstore.GetKvListWith(ctx, "/test/")
	if err != nil || len(kvList) != 2 {
		t.Fatalf("GetKvList failed: got %v, err: %v", kvList, err)
	}

	// 5. Delete
	err = kvstore.DeleteWith(ctx, "/test/key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, found, _ = kvstore.GetWith(ctx, "/test/key1")
	if found {
		t.Fatalf("expected /test/key1 to be deleted")
	}

	// 6. DeleteWithPrefix
	err = kvstore.DeleteWithPrefixWith(ctx, "/test/")
	if err != nil {
		t.Fatalf("DeleteWithPrefix failed: %v", err)
	}
	list, _ = kvstore.GetListWith(ctx, "/test/")
	if len(list) != 0 {
		t.Fatalf("expected 0 items after prefix delete, got %d", len(list))
	}
}

func TestMemoryStoreChildList(t *testing.T) {
	m := NewMemoryStore()
	for _, k := range []string{
		"/ns/x/infra/a", "/ns/x/infra/a/node/n1", "/ns/x/infra/a/nlb/l1",
		"/ns/x/infra/a-b", "/ns/x/infra/a-b/node/n1",
		"/ns/x/infra/orphan/nlb/l9",
		"/ns/x/infra/zz",
		"/ns/x/infrax/other", "/ns/x/resources/vNet/v1",
	} {
		if err := m.Put(k, "v"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.GetChildList("/ns/x/infra/")
	if err != nil {
		t.Fatal(err)
	}
	want := []kvstore.ChildKey{{Name: "a", HasRecord: true}, {Name: "a-b", HasRecord: true}, {Name: "orphan"}, {Name: "zz", HasRecord: true}}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("child %d: got %v want %v", i, got[i], want[i])
		}
	}
	if none, _ := m.GetChildList("/ns/x/k8scluster/"); len(none) != 0 {
		t.Fatalf("expected no children, got %v", none)
	}
}
