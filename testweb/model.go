package main

import (
	"database/sql"
	"log"
	"fmt"
	"encoding/xml"

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
	result += fmt.Sprintf("%s(%s) running?%t\n", p.Name, p.IpAddress, p.Existing)
	for _, vmPtr:= range p.VirtualMachines{
		result += fmt.Sprintf("%s\n",vmPtr)
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
	VNCAddress string
	VNCPort    string
	VirDomain libvirt.VirDomain
}

func (this * VirtualMachine) String() string {
	return fmt.Sprintf("%s %s", this.Name, this.VNCAddress)

}


/* cached VirConnection */
/* IpAddress => VirConnection */
var ipaddressConnectionCache = make(map[string]libvirt.VirConnection)


/* map vm ID(created in database) to VirtualMachine*/

var mapVMIDtoVirtualMachine map[int]*VirtualMachine


var numLiveHost int


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
	/*
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
       */
	return hosts

}


func readLibvirt(hosts []*PhysicalMachine) {
	/* get libvirt connections */
	numLiveHost = 0

	/* use this type in chanStruct */
	type connResult struct {
		host *PhysicalMachine
		conn libvirt.VirConnection
		existing bool
	}
	connChan := make(chan connResult)
	var numGoroutines = 0

	for _, host := range(hosts) {
		conn, ok := ipaddressConnectionCache[host.IpAddress]
		if ok == false {
			numGoroutines ++
			go func(host *PhysicalMachine){
				conn, err := libvirt.NewVirConnection("qemu+ssh://root@" + host.IpAddress + "/system")
				if err != nil {
					checkErr(err,"Can not connect to remove libvirt")
					host.Existing = false
					connChan <- connResult{host:host,existing:false}
				}
				connChan <- connResult{host:host,conn:conn,existing:true}
			}(host)
		} else  {
			/* existing a conn which is alive */
			if ok ,_ := conn.IsAlive();ok {
				host.VirConn =  conn
				host.Existing = true
				numLiveHost ++
			/* existing a conn which is dead */
			} else {
				host.Existing = false
				delete(ipaddressConnectionCache, host.IpAddress)
				/* TODO ?if close the connectin */
				conn.CloseConnection()
			}
		}
	}

	for i:=0;i < numGoroutines ;i++{
		r := <-connChan
		if r.existing{
			r.host.VirConn = r.conn
			r.host.Existing = true
			ipaddressConnectionCache[r.host.IpAddress] = r.conn
			numLiveHost ++
		}
	}


	/* all the PhysicalMachines are ready, VirConnection was connected now */
	/* receive data from VirConnections */

	type VNCinfo struct {
                VNCPort string `xml:"port,attr"`
        }
        type Devices struct {
                Graphics VNCinfo `xml:"graphics"`
        }
        type xmlParseResult struct {
                Name string    `xml:"name"`
                UUID string    `xml:"uuid"`
		Devices  Devices `xml:"devices"`
        }

	done := make(chan bool)
	for _, host := range(hosts) {
		if host.Existing {
			go func(host *PhysicalMachine){
				domains, _ := host.VirConn.ListAllDomains()
				for _, virdomain := range domains {
					v := xmlParseResult{}
					xmlData, _ := virdomain.GetXMLDesc()
					xml.Unmarshal([]byte(xmlData), &v)
					/* if VNCport is -1, this means the domain is closed */
					var active = false
					var vncAddress = ""
					var vncPort = ""
					if (v.Devices.Graphics.VNCPort != "-1") {
						active = true
						vncAddress = host.IpAddress
						vncPort =  v.Devices.Graphics.VNCPort
					}
					vm := VirtualMachine{UUIDString:v.UUID, Name:v.Name, VirDomain:virdomain, Active:active, VNCAddress:vncAddress, VNCPort:vncPort}
					host.VirtualMachines = append(host.VirtualMachines, &vm)
				}
				done <- true
			}(host)
		}

	}
	/* wait for all ListAllDomains finish */
	for i:=0; i< numLiveHost ; i++ {
		<-done
	}

}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}
