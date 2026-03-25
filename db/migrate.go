package main

import (
	"cetus-demo/model"
	"log"

	"github.com/JackDPro/cetus/provider"
)

func main() {
	mysql := provider.GetOrm()
	tables := []interface{}{
		&model.User{},
	}
	for _, table := range tables {
		err := mysql.Db.AutoMigrate(table)
		if err != nil {
			log.Fatalf("create %T table failed: %v\n", table, err)
		}
	}
}
