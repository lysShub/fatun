package itun

import (
	"itun/vendor/github.com/boltdb/bolt"
	"sync"
)

type db struct {
	// config database
	// userConfig, proxyConfig, proxHistory
	db *bolt.DB // config.db

	// 内存中的
	userConfig  map[string]string // 存放用户信息配置、服务器配置等信息
	proxyConfig map[string]string // 存放用户手动配置的代理信息, 支持通配符
	proxHistory map[string]string // 存放自动代理信息, 完全匹配

	sync.RWMutex
}

func openDb(path string) (db *db, err error) {
	var bdb *bolt.DB
	if bdb, err = bolt.Open(dbPath, 0600, nil); err != nil {
		return nil, err
	}

	db.db = bdb

	return
}

func (d *db) loadToMem() error {
	d.Lock()
	defer d.Unlock()

	d.db.Update(func(tx *bolt.Tx) error {
		for _, dbname := range []string{"userConfig", "proxyConfig", "proxHistory"} {

			if b, err := tx.CreateBucketIfNotExists([]byte(dbname)); err != nil {
				return err
			} else {
				c := b.Cursor()
			lookup:
				for k, v := c.First(); k != nil; k, v = c.Next() {
					if len(k) == 0 || len(v) == 0 {
						if err = c.Delete(); err != nil {
							return err
						} else {
							goto lookup
						}
					}

					switch dbname {
					case "userConfig":
						d.userConfig[string(k)] = string(v)
					case "proxyConfig":
						d.proxyConfig[string(k)] = string(v)
					case "proxHistory":
						d.proxHistory[string(k)] = string(v)
					}

				}
			}
		}

		return nil
	})
	return nil
}

func (d *db) SetuserConfig(k, v string) error {
	return nil
}

func (d *db) SetproxyConfig(k, v string) error {
	return nil
}

func (d *db) SetproxHistory(k, v string) error {
	return nil
}
