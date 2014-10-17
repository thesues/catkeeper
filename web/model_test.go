package main

import (
	"database/sql"
	"fmt"
	"github.com/thesues/catkeeper/libvirt"

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
	for _, c:= range ipaddressConnectionCache.Items() {
		c := c.(libvirt.VirConnection)
		c.CloseConnection()
	}
}

func TestRescanIPAddress(t *testing.T) {
	db, err := sql.Open("sqlite3", "/tmp/post_db.bin")
	if err != nil {
		checkErr(err, "open database failed")
	}
	defer db.Close()
	RescanIPAddress(db)
}
