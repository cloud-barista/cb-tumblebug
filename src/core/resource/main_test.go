package resource

import (
	"os"
	"testing"

	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvstore"
	"github.com/cloud-barista/cb-tumblebug/src/kvstore/kvtest"
)

func TestMain(m *testing.M) {
	memStore := kvtest.NewMemoryStore()
	cleanup := kvstore.SetTestStore(memStore)
	defer cleanup()

	code := m.Run()
	os.Exit(code)
}
