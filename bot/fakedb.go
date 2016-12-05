package bot

import "github.com/moira-alert/notifier"

type testDatabase struct {
	conn *notifier.DbConnector
}

func (db *testDatabase) init() {
	c := db.conn.Pool.Get()
	defer c.Close()
	c.Do("FLUSHDB")
}
