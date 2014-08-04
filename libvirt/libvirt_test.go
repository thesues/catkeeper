package libvirt
import (
	"testing"
	"fmt"
	"io/ioutil"
	"errors"
	"time"
)

func TestNewVirConnection(t *testing.T) {
	con, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer con.CloseConnection()

}

func TestListAllDomains(t *testing.T) {
	con, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer con.CloseConnection()
	domainList,_ := con.ListAllDomains()
	for _ , i := range domainList {
		fmt.Printf("is active? %t\n", i.IsActive())
		name, err := i.GetName()
		if err != nil {
			t.Error(err)
			return
		}

		uuidName, err := i.GetUUIDString()
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Printf(uuidName)
		fmt.Printf(name)

	}

}

func TestActiveDomainList(t *testing.T) {
	con, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer con.CloseConnection()

	domainList, err := con.ActiveDomainList()
	if (err != nil) {
		t.Error(err)
	}
	for _ , i := range domainList {
		fmt.Printf("is active? %t\n", i.IsActive())
		name, err := i.GetName()
		if err != nil {
			t.Error(err)
			return
		}

		uuidName, err := i.GetUUIDString()
		if err != nil {
			t.Error(err)
			return
		}
		i.Free()
		fmt.Printf(uuidName)
		fmt.Printf(name)
	}
}

func TestGetXml(t *testing.T) {
	con, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer con.CloseConnection()
	domainList,err := con.ListAllDomains()
	for _ , i := range domainList {
		xml,_ := i.GetXMLDesc()
		_ = xml
		i.Free()
	}
}

func TestStorageVolCreateXML(t *testing.T) {
	conn, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	xml, err := ioutil.ReadFile("./volume.xml")
	if err != nil {
		t.Error(err)
		return
	}

	pool,err := conn.StoragePoolLookupByName("default")
	if err != nil {
		t.Error(err)
	}
	defer pool.Free()

	volume, err := pool.StorageVolCreateXML(string(xml), 1)
	if err != nil {
		t.Error(err)
	}
	defer volume.Free()

	fmt.Println(volume.GetPath())
	volume.Delete()
}


func TestCreateAndBootNewDomain(t *testing.T) {
	conn, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	domainXML, err := ioutil.ReadFile("./domain.xml")
	if err != nil {
		t.Error(err)
		return
	}
	//Create disk first
	volumeXML, err := ioutil.ReadFile("./volume.xml")
	if err != nil {
		t.Error(err)
		return
	}

	pool,err := conn.StoragePoolLookupByName("default")
	if err != nil {
		t.Error(err)
	}
	defer pool.Free()

	volume, err := pool.StorageVolCreateXML(string(volumeXML), 1)
	if err != nil {
		t.Error(err)
	}
	defer volume.Free()
	defer volume.Delete()

	domain, err := conn.CreateAndBootNewDomain(string(domainXML))
	if err != nil {
		t.Error(err)
		return
	}
	defer domain.Free()
	//TODO delete the vi
	defer domain.Delete()
	defer domain.Destroy()

}

func TestStreamTransfer(t *testing.T) {
	conn, err := NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()
	// Test volume pool
	// create vol from pool and Upload
	var pool VirStoragePool
	pool, err = conn.StoragePoolLookupByName("boot-scratch")
	if err != nil {
		// pool not existed
		// create on pool named "boot-scrath"
		// TODO
		fmt.Println("pool not exist")
		poolXML, _:= ioutil.ReadFile("./pool.xml")
		pool, err = conn.StoragePoolDefineXML(string(poolXML))
		if err != nil {
			t.Error(err)
			return
		}
	} else {
		//has the pool
	}
	//do not want to use pool.Delete
	//because there might be other volumes are using the storage pool
	defer pool.Free()

	//is not active, active it
	if pool.IsActive() == false {
		if err := pool.Create();err != nil {
			t.Error(err)
			return
		}
	}

	//create volume
	dataXML, _:= ioutil.ReadFile("./volume_data.xml")
	volume, err := pool.StorageVolCreateXML(string(dataXML),0)
	if err != nil {
		return
	}
	defer volume.Free()
	defer volume.Delete()

	//display path of volume
	path,_ := volume.GetPath()
	fmt.Println("I got " + path)

	stream, err := conn.StreamNew()
	if err != nil {
		t.Error(err)
		return
	}
	defer stream.Free()

	//read data file
	data,_ := ioutil.ReadFile("./data")
	err = StorageVolUpload(volume, stream, 0, uint64(len(data)))
	if err != nil {
		t.Error(err)
		return
	}
	//transfter volume
	fmt.Println("Sending Data...")
	remain := len(data)
	sent := 0
	offset := 0
	for remain > 0 {
		sent = stream.Send(data[offset:], remain)
		if sent < 0 {
			stream.Abort()
			t.Error(errors.New("Stream Send return 0"))
			return
		}
		if sent == 0 {
			break;
		}
		remain -= sent
		offset += sent

	}
	err = stream.Finish()
	if err != nil {
		t.Error(err)
	}
	fmt.Println("Finish Send Data...")

}


/* reboot event monitor */

func monitorRebootcallback(c VirConnection, d VirDomain) {
	fmt.Println("I see")
}

func monitorLifecallback(c VirConnection, d VirDomain , event int, detail int) {
	fmt.Printf("%d happens",event)
}


func TestEventMonitor(t *testing.T) {
	EventRegisterDefaultImpl()
	go func(){
		for {
		EventRunDefaultImpl()
	}}()

	conn, err := NewVirConnection("qemu+ssh:///system")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer conn.CloseConnection()

	var regId int

	domain, err := conn.LookupByName("asdf")
	if (err != nil) {
		t.Error(err)
		return
	}
	defer domain.Free()


	regId = ConnectDomainEventRegister(conn, domain,VIR_DOMAIN_EVENT_ID_REBOOT, (GenericCallBackType)(monitorRebootcallback))
	if regId == -1 {
		return
	}
	fmt.Println(regId)

	regId = ConnectDomainEventRegister(conn, domain,VIR_DOMAIN_EVENT_ID_LIFECYCLE, (LifeCycleCallBackType)(monitorLifecallback))
	if regId == -1 {
		return
	}
	fmt.Println(regId)

	for {
		time.Sleep(1)
	}
}
