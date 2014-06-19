package libvirt
import (
	"testing"
	"fmt"
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
