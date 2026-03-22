package sql_test

import (
	"testing"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared/contracttest"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestStoreContract(t *testing.T) {
	contracttest.RunStoreContractTests(t, testutil.SetupTestSQLStore)
}
