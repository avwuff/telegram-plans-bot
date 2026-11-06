package dbHelper

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sqlRecorder keeps the SQL that gorm would have run.
type sqlRecorder struct {
	logger.Interface
	sql string
}

func (r *sqlRecorder) LogMode(logger.LogLevel) logger.Interface { return r }

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	// gorm's log output puts quotes around named types like CanAttend. They are still sent to the database as numbers.
	r.sql = strings.ReplaceAll(sql, "'", "")
}

// The attendance statuses decide who sees what, in /going, /notgoing and the calendar feeds.
// This runs in dry run mode, so it never connects to a database.
func Test_eventsByAttendance_statuses(t *testing.T) {
	rec := &sqlRecorder{Interface: logger.Discard}
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "test:test@tcp(127.0.0.1:1)/test", SkipInitializeWithVersion: true}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: rec})
	require.NoError(t, err)
	c := &Connector{db: db}

	_, _ = c.CalendarFeed(55)
	assert.Contains(t, rec.sql, "furryplansattend.userid = 55")
	assert.Contains(t, rec.sql, "CanAttend IN (1,2,20,30)")

	_, _ = c.NotGoingEvents(55)
	assert.Contains(t, rec.sql, "furryplansattend.userid = 55")
	assert.Contains(t, rec.sql, "CanAttend IN (0)")
}
