package main

import (
	"database/sql"
	"log"
	"fmt"

	"errors"
	"dmzhang/catkeeper/libvirt"
	"dmzhang/catkeeper/nmap"
	"dmzhang/catkeeper/utils"

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


/* used to describe mappings between MAC<=>IP */
type SubNet struct {
	MAC string
	IP string
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
	Disks     []string
	VirDomain libvirt.VirDomain

	Connect  libvirt.VirConnection

	/*mapping MAC=>IP*/
	MACMapping []SubNet
}


func (this *VirtualMachine) String() string {
	return fmt.Sprintf("%s %s:%s", this.Name, this.VNCAddress, this.VNCPort)

}


func (this *VirtualMachine) Start() error{
	err := this.VirDomain.Create()
	return err

}

func (this *VirtualMachine) Delete(db *sql.DB) error {

	for _, diskpath := range this.Disks {

		log.Printf("deleteing disk %s", diskpath)
		v, err := this.Connect.LookupStorageVolByPath(diskpath)
		if err != nil {
			log.Printf("%s can not be found by libvirt",diskpath)
			continue
		}
		//delete storage
		v.Delete()
		v.Free()
	}

	//remove domain
	err := this.VirDomain.Delete()
	if err != nil {
		return err
	}

	//remove from database
	_, err = db.Exec("delete from virtualmachine where Id = ?", this.Id)
	if err != nil {
		return err
	}
	//find VM's all mac address, delete all mac<=>ip mappings
	rows, err := db.Query("select MAC from vmmacmapping where VmId = ?", this.Id)
	if err != nil {
		return err
	}
	defer rows.Close()
	var mac string
	for rows.Next() {
		rows.Scan(&mac)
		db.Exec("delete from macipmappingcache where MAC = ?", mac)
	}

	_, err = db.Exec("delete from vmmacmapping where VmId = ?", this.Id)
	if err != nil {
		return err
	}
	return nil
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
var ipaddressConnectionCache = utils.NewSafeMap()



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
				row := db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ? and HostIpAddress = ?",
														vm.UUIDString, vm.HostIpAddress)
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
					row = db.QueryRow("select Id, Owner, Description from virtualmachine where UUIDString = ? and HostIpAddress = ?", vm.UUIDString, vm.HostIpAddress)
					row.Scan(&Id, &Owner, &Description)
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
					/* insert into vm-mac-mapping */
					/* used to clean unused MAC in the future */
					for _ , subnet := range vm.MACMapping {
						if _,err := db.Exec("insert into vmmacmapping(VmId, MAC) values (?,?)", Id,subnet.MAC ); err != nil {
							checkErr(err,"failed to insert vmmacmapping")
						}
					}

				} else {
					/* get registered information of vm */
					vm.Id = Id
					vm.Owner = Owner
					vm.Description = Description
					/* find the cached IP address to refresh MACMapping if vm is active*/
					if !vm.Active {
						continue
					}
					for i, subnet := range vm.MACMapping {
						ip := ""
						row := db.QueryRow("select IP from macipmappingcache where MAC = ?",subnet.MAC)
						if err := row.Scan(&ip);err != nil {
							continue
						}
						vm.MACMapping[i].IP = ip
					}
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
	ok := ipaddressConnectionCache.Check(HostIpAddress)
	if ok == false {
		conn, err = libvirt.NewVirConnection("qemu+ssh://root@" + HostIpAddress + "/system")
		if err != nil {
			return VirtualMachine{},err
		}
		ipaddressConnectionCache.Set(HostIpAddress, conn)
	} else {
		//?How to deal with connection's not alive
		conn = ipaddressConnectionCache.Get(HostIpAddress).(libvirt.VirConnection)
		if ok ,_ := conn.IsAlive();!ok {
			log.Printf("remote %s is not alive", HostIpAddress)
			conn, err = libvirt.NewVirConnection("qemu+ssh://root@" + HostIpAddress + "/system")
			if err != nil {
				ipaddressConnectionCache.Delete(HostIpAddress)
				return VirtualMachine{},err
			}
			/*TODO Write Lock*/
			ipaddressConnectionCache.Set(HostIpAddress, conn)
		}
	}

	domain, err := conn.LookupByUUIDString(UUIDString)
	if err != nil {
		return VirtualMachine{},err
	}
	vm := fillVmData(domain, conn)

	return vm,nil
}


//TODO: in the futhure, use vm := VirtualMachine{};fillvmData(domain,vm)
func fillVmData(domain libvirt.VirDomain, conn libvirt.VirConnection) VirtualMachine {

	xmlData, _ := domain.GetXMLDesc()
	v := utils.ParseDomainXML(xmlData)
	/* if VNCPort is -1, this means the domain is closed */
	var active = false
	var vncPort = ""
	if (v.Devices.Graphics.VNCPort != "-1") {
		active = true
		vncPort =  v.Devices.Graphics.VNCPort
	}

	/* fill MAC Address */
	macMapping := make([]SubNet,0)
	for _, i := range v.Devices.Interface {
		if i.Type == "bridge" {
			macMapping = append(macMapping, SubNet{MAC:i.MAC.Address, IP:"not detected"})
		}
	}

	/* fill Disk info */

	var vmDisks = make([]string,0)
	for _, i := range v.Devices.Disks {
		vmDisks = append(vmDisks, i.Source.Path)
	}

	return VirtualMachine{UUIDString:v.UUID, Name:v.Name, Active:active, VNCPort:vncPort,VirDomain:domain, MACMapping:macMapping, Disks:vmDisks, Connect:conn}
}

func getConnectionFromCacheByIP(ip string) (libvirt.VirConnection, error){
	ok := ipaddressConnectionCache.Check(ip)
	log.Println(ipaddressConnectionCache)
	if ok == false {
		return libvirt.VirConnection{}, errors.New("No connections to remote")
	}
	conn := ipaddressConnectionCache.Get(ip).(libvirt.VirConnection)
	log.Println(conn)
	return conn, nil

}

func readLibvirtPysicalMachine(hosts []*PhysicalMachine) {
	/* get libvirt connections */
	numLiveHost := 0
	var conn libvirt.VirConnection

	/* use this type in chanStruct */
	type connResult struct {
		host *PhysicalMachine
		conn libvirt.VirConnection
		existing bool
	}
	connChan := make(chan connResult)
	var numGoroutines = 0

	for _, host := range(hosts) {
		ok := ipaddressConnectionCache.Check(host.IpAddress)
		if ok == false {
			numGoroutines ++
			go func(host *PhysicalMachine){
				conn, err := libvirt.NewVirConnection("qemu+ssh://root@" + host.IpAddress + "/system")
				if err != nil {
					checkErr(err, fmt.Sprintf("failed to connect to %s", host.IpAddress))
					host.Existing = false
					connChan <- connResult{host:host,existing:false}
					return
				}
				connChan <- connResult{host:host,conn:conn,existing:true}
			}(host)
		} else  {
			/* existing a conn which is alive */
			conn = ipaddressConnectionCache.Get(host.IpAddress).(libvirt.VirConnection)
			if ok ,_ := conn.IsAlive();ok {
				host.VirConn =  conn
				host.Existing = true
				numLiveHost ++
			/* existing a conn which is dead */
			} else {
				log.Printf("remove %s is not alive", host.IpAddress)
				host.Existing = false
				ipaddressConnectionCache.Delete(host.IpAddress)
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
			ipaddressConnectionCache.Set(r.host.IpAddress, r.conn)
			numLiveHost ++
		}
	}



	/* all the PhysicalMachines are ready, VirConnection was connected now */
	/* receive data from VirConnections */

	done := make(chan bool)
	for _, host := range(hosts) {
		if host.Existing == false {
			continue
		}

		go func(host *PhysicalMachine){
			domains, _ := host.VirConn.ListAllDomains()
			for _, virdomain := range domains {
				vm := fillVmData(virdomain, conn)
				vm.HostIpAddress = host.IpAddress
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
	/* wait for all ListAllDomains finish */
	for i:=0; i< numLiveHost ; i++ {
		<-done
	}

}

func RescanIPAddress(db *sql.DB) {

	hosts := getListofPhysicalMachineAndVirtualMachine(db)

	for _, myIP:= range utils.LocalIPs() {
		/* scan */
		mapping,err := nmap.Nmap(myIP)
		if err != nil {
			checkErr(err,"nmap failed")
			continue
		}

		/* match and insert into database */
		for _, host := range hosts {
			if host.Existing == false{
				continue
			}
			for _, vm := range host.VirtualMachines {
				for _ ,subnet := range vm.MACMapping {
					ip := ""
					_, ok := mapping[subnet.MAC]
					if ok == false{
						continue
					}
					err := db.QueryRow("select IP from  macipmappingcache where MAC = ?", subnet.MAC).Scan(&ip)
					switch {
						case err ==  sql.ErrNoRows:
							/*insert*/
							db.Exec("insert into macipmappingcache(IP, MAC) values(?,?)", mapping[subnet.MAC], subnet.MAC)
						case err != nil:
							checkErr(err,"failed to select on macipmappingcache")
						default:
							if ip != mapping[subnet.MAC] {
								db.Exec("udpate macipmappingcache set IP = ? wheree MAC = ?",mapping[subnet.MAC], subnet.MAC)
							}

					}
				}
			}
		}
	}

}


func checkErr(err error, msg string) {
	if err != nil {
		log.Print(msg, err)
	}
}
