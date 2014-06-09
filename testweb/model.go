package main

import (
	"database/sql"
	"log"
	"fmt"

	"dmzhang/libvirt"
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
	VirConn    *libvirt.VirConnection
}

func (p *PhysicalMachine) String() string{
	var result = ""
	for _, host := range p{
		result += fmt.Sprintf("%s(%s)", p.Name, p.IpAddress)
		for _, vmPtr := range host.VirtualMachines {
			result += fmt.Sprintf(*vmPtr)
		}

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
	MyVirDomain libvirt.VirDomain
}

var db *sql.DB
var ipaddressConnectionMap = make(map[string]*libvirt.VirConnection)
var vmidVirtualMachineMap  = make(map[int]*VirtualMachine)

func getListofPhysicalMachine() []*PhysicalMachine {
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
		hosts = append(hosts, &PhysicalMachine{Name:Name, Id:Id, IpAddress:IpAddress, Description:Description, VirConn:nil, Existing:false})
	}

	/* read libvirt and set exist flag in PhysicalMachine*/
	readLibvirt(hosts)

	/* read virtualmachine from database */
	for _, host := range hosts {
		if host.Existing == true {
			for _, vm := range host.VirtualMachines{
				row := db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ?", vm.UUIDString)
				if err = row.Scan(&Id, &Owner, &Description); err != nil {
					/* not registered vm */
					Owner = "no one"
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
				/* fill the vmidVirtualMachineMap */
				vmidVirtualMachineMap[vm.Id] = &vm;
			}
		}

	}
	/* create hash map for all the hosts*/
	return hosts

}

func readLibvirt(hosts []*PhysicalMachine) {
	/* get libvirt connections */
	for _, host := range(hosts) {
		connPtr:= ipaddressConnectionMap[host.IpAddress]

		if connPtr == nil {
			conn, err := libvirt.NewVirConnection("qemu+ssh://root@" + host.IpAddress + "/system")
			if err != nil {
				checkErr(err,"Can not connect to remove libvirt")
				host.Existing = false
				continue
			}
			ipaddressConnectionMap[host.IpAddress] = &conn
			host.VirConn =  &conn
			host.Existing = true
		} else  {
			/* existing a conn */
			host.VirConn =  connPtr
			host.Existing = true
		}
	}

	/* receive data from VirConnections */
	for _, host := range(hosts) {
		if host.Existing {
			domains, _ := host.VirConn.ListAllDomains()
			for _, virdomain := range domains {
				name, _ := virdomain.GetName()
				uuid, _ := virdomain.GetUUIDString()
				vm := VirtualMachine{UUIDString:uuid, Name:name, MyVirDomain:virdomain}
				host.VirtualMachines = append(host.VirtualMachines, &vm)
			}
		}

	}

}

/* read libvirt */
func main() {
	var err error
	db, err = sql.Open("sqlite3", "/tmp/post_db.bin")
	if err != nil {
		checkErr(err, "open database failed")
	}
	defer db.Close()

	pm := getListofPhysicalMachine()
	/* display */
	fmt.Println(pm)
	/* release domains */
	for _, vm in range vmidVirtualMachineMap {
		vm.DomainFree()
	}
	/* release connections */
	for _, host in range ipaddressConnectionMap {
		host.
		host.VirDomain.CloseConnection()
	}
	/* goroutins */
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}
