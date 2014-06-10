package main

import (
	"database/sql"
	"log"
	"fmt"

	"dmzhang/catkeeper/libvirt"
	_ "github.com/mattn/go-sqlite3"
)

type PhysicalMachine struct {
	/* database */
	Id          int
	IpAddress   string
	Description string
	Name	    string

	/* libvirt */
	Existing   bool
	VirtualMachines []*VirtualMachine
	VirConn    libvirt.VirConnection
}

func (p *PhysicalMachine) String() string{
	var result = ""
	result += fmt.Sprintf("%s(%s)\n", p.Name, p.IpAddress)
	for _, vmPtr:= range p.VirtualMachines{
		result += fmt.Sprintf("%v\n",*vmPtr)

	}

	return result
}

type VirtualMachine struct {
	/* database */
	Id          int
	UUIDString  string /*set by libvirt*/
	Owner       string
	Description string

	/* libvirt */
	Name string
	Active bool
	VirDomain libvirt.VirDomain
}


/* cached VirConnection */
/* IpAddress => VirConnection */
var ipaddressConnectionCache = make(map[string]libvirt.VirConnection)


/* map vm ID(created in database) to VirtualMachine*/

var mapVMIDtoVirtualMachine map[int]*VirtualMachine


func getListofPhysicalMachine(db *sql.DB) []*PhysicalMachine {
	/* read database to physicalmachine*/
	var (
		hosts       []*PhysicalMachine
		Id          int
		IpAddress   string
		Description string
		Name        string
		rows        *sql.Rows
		Owner       string
		err         error
	)
	rows, err = db.Query("select Id,Name,IpAddress, Description from physicalmachine")
	if err != nil {
		checkErr(err, "failed to query select * from physicalmachine")
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		if err = rows.Scan(&Id, &Name, &IpAddress, &Description); err != nil {
			checkErr(err, "row scan failed")
			return nil
		}
		hosts = append(hosts, &PhysicalMachine{Name:Name, Id:Id, IpAddress:IpAddress, Description:Description, Existing:false})
	}

	/* read libvirt and set exist flag in PhysicalMachine*/
	readLibvirt(hosts)

	/* read virtualmachine from database */
	mapVMIDtoVirtualMachine = make(map[int]*VirtualMachine)
	for _, host := range hosts {
		if host.Existing == true {
			for _, vm := range host.VirtualMachines{
				row := db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ?", vm.UUIDString)
				if err = row.Scan(&Id, &Owner, &Description); err != nil {
					/* not registered vm */
					Owner = "no one is using me"
					Description = "I am new vm"
					stmt, _ := db.Prepare("insert into virtualmachine(Owner, Description, UUIDString) values (?, ?, ?)")
					_, err = stmt.Exec(Owner, Description, vm.UUIDString)
					if err != nil {
						checkErr(err, "failed to create info for vm")
						continue
					}
					/* re-select again*/
					row = db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ?", vm.UUIDString)
					rows.Scan(&Id, &Owner, &Description)
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
				} else {
					/*get registered information of vm*/
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
				}
				/* create a map for all the hosts*/
				mapVMIDtoVirtualMachine[vm.Id] = vm
			}
		}

	}
	/* delete absoleted vm from database  */
	rows, err = db.Query("select Id from virtualmachine")
	if err != nil {
		checkErr(err,"select Id from virtualmachine failed")
		return hosts
	}
	for rows.Next() {
		rows.Scan(&Id)
		if _,ok := mapVMIDtoVirtualMachine[Id]; ok == true {
			continue
		}else {
			db.Exec("delete from virtualmachine where Id=?",Id)
		}
	}
	return hosts

}

func readLibvirt(hosts []*PhysicalMachine) {
	/* get libvirt connections */
	for _, host := range(hosts) {
		conn,ok := ipaddressConnectionCache[host.IpAddress]
		if ok == false {
			conn, err := libvirt.NewVirConnection("qemu+ssh://root@" + host.IpAddress + "/system")
			if err != nil {
				checkErr(err,"Can not connect to remove libvirt")
				host.Existing = false
				continue
			}
			host.VirConn =  conn
			host.Existing = true
			ipaddressConnectionCache[host.IpAddress] = conn
		} else  {
			/* existing a conn which is alive */
			if ok ,_ := conn.IsAlive();ok {
				host.VirConn =  conn
				host.Existing = true
			/* existing a conn which is dead */
			} else {
				host.Existing = false
				delete(ipaddressConnectionCache, host.IpAddress)
				/* TODO ?if close the connectin */
				conn.CloseConnection()
			}
		}

	}

	/* receive data from VirConnections */
	for _, host := range(hosts) {
		if host.Existing {
			domains, _ := host.VirConn.ListAllDomains()
			for _, virdomain := range domains {
				name, _ := virdomain.GetName()
				uuid, _ := virdomain.GetUUIDString()
				active := virdomain.IsActive()
				vm := VirtualMachine{UUIDString:uuid, Name:name, VirDomain:virdomain, Active:active}
				host.VirtualMachines = append(host.VirtualMachines, &vm)
			}
		}

	}

}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}
