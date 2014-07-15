package main

import (
	"database/sql"
	"log"
	"fmt"
	"encoding/xml"

	"errors"
	"sync"
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
	HostIpAddress string

	/* libvirt */
	Name string
	Active bool
	VNCAddress string
	VNCPort    string
	VirDomain libvirt.VirDomain
}

func (this *VirtualMachine) String() string {
	return fmt.Sprintf("%s %s:%s", this.Name, this.VNCAddress, this.VNCPort)

}


func (this *VirtualMachine) Start() error{
	err := this.VirDomain.Create()
	return err

}


func (this *VirtualMachine) Stop() error {
	err := this.VirDomain.Shutdown()
	return err
}


func (this *VirtualMachine) ForceStop() error {
	err := this.VirDomain.Destroy()
	return err
}

func (this *VirtualMachine) Free() error {
	err := this.VirDomain.Free()
	return err
}

func (this *VirtualMachine) UpdateDatabase(db *sql.DB, owner string, description string) error {
	if owner != "" && description != "" {
		_, err := db.Exec("update virtualmachine set Owner=?,Description=? where Id=?", owner, description, this.Id)
		if err!= nil {
			return err
		}else {
			return nil
		}
	} else {//can not be updated
		return errors.New("owner and description must have values")
	}
}

// global variables: do not like global variables
// cached VirConnection
//IpAddress => VirConnection
var ipaddressConnectionCache = make(map[string]libvirt.VirConnection)

// cacheMutex is used to protect the ipaddressConnectionCache map from multiple users 
var cacheMutex = &sync.Mutex{}

/* map vm ID(created in database) to VirtualMachine*/
// not used any more
//var mapVMIDtoVirtualMachine map[int]*VirtualMachine



func getVirtualMachine(db *sql.DB, Id int) *VirtualMachine {
	var (
		UUIDString string
		Owner  string
		Description string
		HostIpAddress string
	)

	row  := db.QueryRow("select Id,UUIDString,Owner, Description, HostIpAddress from virtualmachine where Id=?",Id)
	if err := row.Scan(&Id, &UUIDString, &Owner, &Description, &HostIpAddress); err !=nil {
		checkErr(err, "failed to scan in getVirtualMachine")
		return nil
	}
	vm, err :=  readLibvirtVM(HostIpAddress, UUIDString)

	if (err != nil) {
		checkErr(err,"failed to get information from libvirt")
		return nil
	}
	vm.Id = Id
	vm.Owner = Owner
	vm.Description = Description
	vm.HostIpAddress = HostIpAddress
	return &vm
}

func getListofPhysicalMachineAndVirtualMachine(db *sql.DB) []*PhysicalMachine {
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
	readLibvirtPysicalMachine(hosts)


	/* read virtualmachine from database */
	for _, host := range hosts {
		if host.Existing == true {
			for _, vm := range host.VirtualMachines{
				row := db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ?", vm.UUIDString)
				if err = row.Scan(&Id, &Owner, &Description); err != nil {
					/* not registered vm */
					Owner = "no one is using me"
					Description = "I am new vm"
					stmt, _ := db.Prepare("insert into virtualmachine(Owner, Description, UUIDString,HostIpAddress) values (?, ?, ?, ?)")
					_, err = stmt.Exec(Owner, Description, vm.UUIDString, host.IpAddress)
					if err != nil {
						checkErr(err, "failed to create info for vm")
						continue
					}
					/* re-select again*/
					row = db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ?", vm.UUIDString)
					row.Scan(&Id, &Owner, &Description)
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
				} else {
					/*get registered information of vm*/
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
				}
			}
		}

	}
	// delete absoleted vm from database
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

func readLibvirtVM(HostIpAddress string, UUIDString string) (VirtualMachine, error) {
	var conn libvirt.VirConnection
	var err error
	cacheMutex.Lock()
	conn, ok := ipaddressConnectionCache[HostIpAddress]
	cacheMutex.UnLock()
	if ok == false {
		conn, err = libvirt.NewVirConnection("qemu+ssh://root@" + HostIpAddress + "/system")
		if err != nil {
			return VirtualMachine{},err
		}
		/*TODO Write Lock*/
		cacheMutex.Lock()
		ipaddressConnectionCache[HostIpAddress] = conn
		cacheMutex.Unlock()
	} else {
		//?How to deal with connection's not alive
		if ok ,_ := conn.IsAlive();!ok {
			log.Println("Not alive")
			conn, err = libvirt.NewVirConnection("qemu+ssh://root@" + HostIpAddress + "/system")
			if err != nil {
				cacheMutex.Lock()
				delete(ipaddressConnectionCache,HostIpAddress)
				cacheMutex.Unlock()
				return VirtualMachine{},err
			}
			/*TODO Write Lock*/
			cacheMutex.Lock()
			ipaddressConnectionCache[HostIpAddress] = conn
			cacheMutex.Unlock()
		}
	}

	domain, err := conn.LookupByUUIDString(UUIDString)
	if err != nil {
		return VirtualMachine{},err
	}
	vm := fillVmData(domain)
	return vm,nil
}


//TODO: in the futhure, use vm := VirtualMachine{};fillvmData(domain,vm)
func fillVmData(domain libvirt.VirDomain) VirtualMachine {
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
	v := xmlParseResult{}
	xmlData, _ := domain.GetXMLDesc()
	xml.Unmarshal([]byte(xmlData), &v)
	/* if VNCPort is -1, this means the domain is closed */
	var active = false
	var vncPort = ""
	if (v.Devices.Graphics.VNCPort != "-1") {
		active = true
		vncPort =  v.Devices.Graphics.VNCPort
	}
	return VirtualMachine{UUIDString:v.UUID, Name:v.Name, Active:active, VNCPort:vncPort,VirDomain:domain}
}

func readLibvirtPysicalMachine(hosts []*PhysicalMachine) {
	/* get libvirt connections */
	numLiveHost := 0

	/* use this type in chanStruct */
	type connResult struct {
		host *PhysicalMachine
		conn libvirt.VirConnection
		existing bool
	}
	connChan := make(chan connResult)
	var numGoroutines = 0

	for _, host := range(hosts) {
		cacheMutex.Lock()
		conn, ok := ipaddressConnectionCache[host.IpAddress]
		cacheMutex.UnLock()
		if ok == false {
			numGoroutines ++
			go func(host *PhysicalMachine){
				conn, err := libvirt.NewVirConnection("qemu+ssh://root@" + host.IpAddress + "/system")
				if err != nil {
					checkErr(err,"Can not connect to libvirt")
					host.Existing = false
					connChan <- connResult{host:host,existing:false}
					return
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
				cacheMutex.Lock()
				delete(ipaddressConnectionCache, host.IpAddress)
				cacheMutex.UnLock()
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
			/*Write Lock*/
			cacheMutex.Lock()
			ipaddressConnectionCache[r.host.IpAddress] = r.conn
			cacheMutex.Unlock()
			numLiveHost ++
		}
	}



	/* all the PhysicalMachines are ready, VirConnection was connected now */
	/* receive data from VirConnections */

	done := make(chan bool)
	for _, host := range(hosts) {
		if host.Existing {
			go func(host *PhysicalMachine){
				domains, _ := host.VirConn.ListAllDomains()
				for _, virdomain := range domains {
					vm := fillVmData(virdomain)
					if vm.Active == true {
						vm.VNCAddress = host.IpAddress
					}
					//will not have any operations on vm, virdomain could be freeed
					virdomain.Free()
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
		log.Print(msg, err)
	}
}
