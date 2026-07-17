package store_test

import (
	"testing"

	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/internal/testcontract"
)

func TestMemoryStoreContract(t *testing.T) {
	testcontract.RunStoreContract(t, func() store.Store {
		return store.NewMemoryStore()
	})
}
