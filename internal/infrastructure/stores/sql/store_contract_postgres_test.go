package sql_test

import (
	"testing"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared/contracttest"
	"github.com/movebigrocks/platform/internal/testutil"
)

func TestStoreContractPostgres(t *testing.T) {
	contracttest.RunStoreContractTests(t, testutil.SetupTestPostgresStore)
}
