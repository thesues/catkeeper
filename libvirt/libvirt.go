package libvirt

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

/*from github.com/alexzorin/libvirt-go*/

/*
#cgo LDFLAGS: -lvirt -ldl
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>
*/
import "C"


type VirConnection struct {
	ptr C.virConnectPtr
}

type VirDomain struct {
	ptr C.virDomainPtr
}

func GetLastError() string {
	err := C.virGetLastError()
	errMsg := fmt.Sprintf("[Code-%d] [Domain-%d] %s",
		err.code, err.domain, C.GoString(err.message))
	C.virResetError(err)
	return errMsg
}

/* virtual connection */
func NewVirConnection(uri string) (VirConnection, error) {
	cUri := C.CString(uri)
	defer C.free(unsafe.Pointer(cUri))
	ptr := C.virConnectOpen(cUri)
	if ptr == nil {
		return VirConnection{}, errors.New(GetLastError())
	}
	obj := VirConnection{ptr: ptr}
	return obj, nil
}


func (c *VirConnection) CloseConnection() (int, error) {
	result := int(C.virConnectClose(c.ptr))
	if result == -1 {
		return result, errors.New(GetLastError())
	}
	return result, nil
}

func (c *VirConnection) NumOfActiveDomains() (int, error) {
	result := int(C.virConnectNumOfDomains(c.ptr))
	if result == -1 {
		return 0, errors.New(GetLastError())
	}
	return result, nil
}

func (c *VirConnection) NumOfInActiveDomains() (int, error) {
	result := int(C.virConnectNumOfDefinedDomains(c.ptr))
	if result == -1 {
		return 0, errors.New(GetLastError())
	}
	return result, nil
}

func (c *VirConnection) ActiveDomainList() ([]VirDomain, error) {
	var cDomainsIds [512](C.int)
	var i int
	cDomainsPointer := unsafe.Pointer(&cDomainsIds)
	cNumDomains := C.virConnectListDomains(c.ptr, (*C.int)(cDomainsPointer), 512)
	var err error
	if int(cNumDomains) == -1 {
		return nil, errors.New(GetLastError())
	}
	activeDomainList := make([]VirDomain, int(cNumDomains))
	for i = 0; i < int(cNumDomains); i++ {
		activeDomainList[i].ptr = C.virDomainLookupByID(c.ptr, C.int(cDomainsIds[i]))
		if activeDomainList[i].ptr == nil {
			err = errors.New(GetLastError())
			break
		}
	}

	return activeDomainList[:i], err

}


func (c *VirConnection) IsAlive() (bool, error) {
	switch result := int(C.virConnectIsAlive(c.ptr));result {
	case -1:
		return false, errors.New(GetLastError())
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, nil
	}

}

func (c *VirConnection) InActiveDomainList() ([]VirDomain, error) {
	var cDomainsNames [512](*C.char)
	var err error
	var i int
	cDomainsPointer := unsafe.Pointer(&cDomainsNames)
	numDomains := C.virConnectListDefinedDomains(c.ptr, (**C.char)(cDomainsPointer), 512)
	if numDomains == -1 {
		return nil, errors.New(GetLastError())
	}

	inActiveDomainList := make([]VirDomain, numDomains)

	for i = 0; i < int(numDomains); i++ {
		inActiveDomainList[i].ptr = C.virDomainLookupByName(c.ptr, (*C.char)(cDomainsNames[i]))
		C.free(unsafe.Pointer(cDomainsNames[i]))
		if inActiveDomainList[i].ptr == nil {
			err = errors.New(GetLastError())
			break
		}
	}

	return inActiveDomainList[:i], err

}

func (c *VirConnection) ListAllDomains() ([]VirDomain, error) {
	var cList *C.virDomainPtr
	/* 3 ==  VIR_CONNECT_LIST_DOMAINS_ACTIVE | VIR_CONNECT_LIST_DOMAINS_INACTIVE */
	numDomains := C.virConnectListAllDomains(c.ptr, (**C.virDomainPtr)(&cList),3)

	if numDomains == -1 {
		return nil, errors.New(GetLastError())
	}
	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cList)),
		Len:  int(numDomains),
		Cap:  int(numDomains),
	}
	var domains []VirDomain
	slice := *(*[]C.virDomainPtr)(unsafe.Pointer(&hdr))
	for _, ptr := range slice {
		domains = append(domains, VirDomain{ptr})
	}
	C.free(unsafe.Pointer(cList))
	return domains, nil
}


func (c *VirConnection) LookupByUUIDString(uuid string) (VirDomain,error) {
	var cUUID = C.CString(uuid)
	defer C.free(unsafe.Pointer(cUUID))
	ptr := C.virDomainLookupByUUIDString(c.ptr, cUUID)
	if ptr == nil {
		return VirDomain{}, errors.New(GetLastError())
	}
	return VirDomain{ptr:ptr}, nil
}

func (c *VirConnection) LookupByName(name string) (VirDomain, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.virDomainLookupByName(c.ptr, cName)
	if ptr == nil {
		return VirDomain{}, errors.New(GetLastError())
	}
	return VirDomain{ptr:ptr}, nil

}

/* virtual domain */
func (d *VirDomain) Create() error {
	result := C.virDomainCreate(d.ptr)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}

func (d *VirDomain) DomainFree() error {
	result := C.virDomainFree(d.ptr)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}

func (d *VirDomain) Destroy() error {
	result := C.virDomainDestroy(d.ptr)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}

func (d *VirDomain) Shutdown() error {
	result := C.virDomainShutdown(d.ptr)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}


func (d *VirDomain) Reboot(flags uint) error {
	result := C.virDomainReboot(d.ptr, C.uint(flags))
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}



func (d *VirDomain) GetUUIDString() (string, error) {
	var cUuid [C.VIR_UUID_STRING_BUFLEN](C.char)
	cuidPtr := unsafe.Pointer(&cUuid)
	result := C.virDomainGetUUIDString(d.ptr, (*C.char)(cuidPtr))
	if result != 0 {
		return "", errors.New(GetLastError())
	}
	return C.GoString((*C.char)(cuidPtr)), nil
}


func (d *VirDomain) GetName() (string, error) {
	cName := C.virDomainGetName(d.ptr)
	if cName == nil {
		return "", errors.New(GetLastError())
	}
	return C.GoString(cName), nil
}

func (d *VirDomain) IsActive() bool {
	result := C.virDomainIsActive(d.ptr)
	if result == -1 {
		return false
	}
	if result == 1 {
		return true
	}
	return false
}


func (d *VirDomain) GetXMLDesc() (string, error) {
	result := C.virDomainGetXMLDesc(d.ptr, C.VIR_DOMAIN_XML_INACTIVE)
	if result == nil {
		return "", errors.New(GetLastError())
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}


