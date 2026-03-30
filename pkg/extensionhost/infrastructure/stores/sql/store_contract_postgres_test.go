package sql_test

import (
	"testing"

	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/shared/contracttest"
	"github.com/movebigrocks/platform/pkg/extensionhost/testutil"
)

func TestStoreContractPostgres(t *testing.T) {
	contracttest.RunStoreContractTests(t, testutil.SetupTestPostgresStore)
}
