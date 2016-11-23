package bot

import (
	"github.com/boltdb/bolt"
)

// IDReader reads ID by login from KV storage
type IDReader interface {
	ReadID(login string) string
}

// IDWriter writes ID by login to KV storage
type IDWriter interface {
	WriteID(login, id string) error
}

// DBCloser closes DB connection
type DBCloser interface {
	Close()
}

// DB implements DB interface
type DB interface {
	IDReader
	IDWriter
	DBCloser
}

type db struct {
	db *bolt.DB
}

// NewDb connects to DB
func NewDb(name string) DB {
	instance := db{}
	var err error
	instance.db, err = bolt.Open(name, 0600, nil)
	if err != nil {
		logger.Fatal(err)
	}
	result := &instance
	result.initBuckets([]string{"users"})
	return result
}

func (d *db) Close() {
	logger.Debug("DB closed")
	d.db.Close()
}

func (d *db) initBuckets(buckets []string) {
	for _, name := range buckets {
		d.db.Update(func(tx *bolt.Tx) error {
			logger.Debugf("Trying to create bucket: %s", name)
			_, err := tx.CreateBucketIfNotExists([]byte(name))
			if err != nil {
				logger.Errorf("Error when creating bucket: %s", err)
				return err
			}
			logger.Debugf("Bucket %s created", name)
			return nil
		})
	}
}

func (d *db) ReadID(login string) string {
	if len(login) > 0 && login[0] == byte('#') {
		result := "@" + login[1:]
		logger.Debugf("Channel %s requested. Returning id: %s", login, result)
		return result
	}
	result := make(chan string)
	logger.Debugf("Starting read ID for login (%s)", login)
	go d.db.View(func(tx *bolt.Tx) error {
		logger.Debug("Transaction started")

		bucket := tx.Bucket([]byte("users"))
		val := bucket.Get([]byte(login))

		logger.Debugf("ID obtained: %s", val)
		result <- string(val)
		close(result)
		return nil
	})

	return <-result
}

func (d *db) WriteID(login, id string) error {
	logger.Debugf("Starting to write ID: %s", id)
	return d.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		logger.Debugf("ID written: %s", id)
		return bucket.Put([]byte(login), []byte(id))
	})
}
