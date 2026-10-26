package db

import (
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// IsUniqueViolation reports whether err is a UNIQUE (or PRIMARY KEY)
// constraint failure, so domain packages can turn it into their own
// "already exists" error without matching message text.
func IsUniqueViolation(err error) bool {
	e, ok := errors.AsType[*sqlite.Error](err)
	if !ok {
		return false
	}
	return e.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || e.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}
