package platformsql

import internalsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"

type DB = internalsql.DB
type DBConfig = internalsql.DBConfig
type Store = internalsql.Store
type SqlxDB = internalsql.SqlxDB

var NewDBWithConfig = internalsql.NewDBWithConfig
var NewStore = internalsql.NewStore
var NewSqlxDB = internalsql.NewSqlxDB
