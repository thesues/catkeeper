package libvirt
import (
	"testing"
	"fmt"
	"io/ioutil"
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
		i.DomainFree()
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
		i.DomainFree()
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

	domain, err := conn.CreateAndBootNewDomain(string(domainXML))
	if err != nil {
		t.Error(err)
		return
	}
	defer domain.DomainFree()
}
