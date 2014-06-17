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

	pm := getListofPhysicalMachine(db)
	// display
	fmt.Println(pm)
	// release domains
	for _, v := range mapVMIDtoVirtualMachine{
		v.VirDomain.DomainFree()
	}
	// release connections
	for _, c:= range ipaddressConnectionCache {
		c.CloseConnection()
	}
}
