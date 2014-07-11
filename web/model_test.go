package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"testing"
)

func TestConnectionAndBuildDabase(t *testing.T) {
	db, err := sql.Open("sqlite3", "/tmp/post_db.bin")
	if err != nil {
		checkErr(err, "open database failed")
	}
	defer db.Close()

	pm := getListofPhysicalMachineAndVirtualMachine(db)
	// display
	fmt.Println(pm)
	// release connections
	for _, c:= range ipaddressConnectionCache {
		c.CloseConnection()
	}
}
