package libvirt

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

/* part code from github.com/alexzorin/libvirt-go */

/*
#cgo LDFLAGS: -lvirt -ldl
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdlib.h>

void EventCallBack(virConnectPtr c, virDomainPtr d, int event, int detail, void * data);
void libvirt_eventcallback_cgo(virConnectPtr c, virDomainPtr d, int event, int detail, void * data);
void VirFreeCallback(void *);
void libvirt_virfreecalback_cgo(void *opaque);
*/
import "C"


const (
      VIR_DOMAIN_EVENT_DEFINED  = int(C.VIR_DOMAIN_EVENT_DEFINED)
      VIR_DOMAIN_EVENT_UNDEFINE = int(C.VIR_DOMAIN_EVENT_UNDEFINED)
      VIR_DOMAIN_EVENT_STARTED  = int(C.VIR_DOMAIN_EVENT_STARTED)
      VIR_DOMAIN_EVENT_SUSPENDE = int(C.VIR_DOMAIN_EVENT_SUSPENDED)
      VIR_DOMAIN_EVENT_RESUMED  = int(C.VIR_DOMAIN_EVENT_RESUMED)
      VIR_DOMAIN_EVENT_STOPPED  = int(C.VIR_DOMAIN_EVENT_STOPPED)
      VIR_DOMAIN_EVENT_SHUTDOWN = int(C.VIR_DOMAIN_EVENT_SHUTDOWN)
)

type VirConnection struct {
	ptr C.virConnectPtr
}

type VirDomain struct {
	ptr C.virDomainPtr
}


type VirStream struct {
	ptr C.virStreamPtr
}

func (c *VirConnection) StreamNew() (VirStream,error) {
	ptr := C.virStreamNew(c.ptr, 0);
	if ptr == nil {
		return VirStream{}, errors.New(GetLastError())
	}
	return VirStream{ptr:ptr}, nil

}


func StorageVolUpload(vol VirStorageVol, s VirStream, offset uint64 , length uint64) error {

	result := C.virStorageVolUpload(vol.ptr, s.ptr, C.ulonglong(offset), C.ulonglong(length), 0)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
}

func (s *VirStream) Send(data []byte, size int) int {
	cBytes := C.virStreamSend(s.ptr, (*C.char)(unsafe.Pointer(&data[0])), C.size_t(size))
	return int(cBytes)
}


func (s *VirStream) Finish() error {
	result := C.virStreamFinish(s.ptr)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
}


func (s *VirStream) Abort() error {
	result := C.virStreamAbort(s.ptr)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
}

func (s *VirStream) Free() error {
	result := C.virStreamFree(s.ptr)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
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


func (c *VirConnection) CreateAndBootNewDomain(xml string)(VirDomain,error) {
	cXml := C.CString(xml)
	defer C.free(unsafe.Pointer(cXml))

	cDomainPtr := C.virDomainDefineXML(c.ptr, cXml)
	if cDomainPtr == nil {
		return VirDomain{}, errors.New(GetLastError())
	}


	result := C.virDomainCreate(cDomainPtr)
	if  result == -1 {
		return VirDomain{}, errors.New(GetLastError())
	}

	return VirDomain{ptr:cDomainPtr},nil
}

func (c *VirConnection) CreateXML(xml string)(VirDomain,error) {
        cXml := C.CString(xml)
        defer C.free(unsafe.Pointer(cXml))

        cDomainPtr := C.virDomainCreateXML(c.ptr, cXml, 0)
        if cDomainPtr == nil {
                return VirDomain{}, errors.New(GetLastError())

        }
	return VirDomain{ptr:cDomainPtr},nil

}

func (c *VirConnection) DefineXML(xml string)(VirDomain, error) {
	cXml := C.CString(xml)
	defer C.free(unsafe.Pointer(cXml))

	cDomainPtr := C.virDomainDefineXML(c.ptr, cXml)
	if cDomainPtr == nil {
		return VirDomain{}, errors.New(GetLastError())
	}
	return VirDomain{ptr:cDomainPtr},nil
}


func (c *VirConnection) StoragePoolLookupByName(name string) (VirStoragePool,error){
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.virStoragePoolLookupByName(c.ptr, cName)
	if ptr == nil {
		return VirStoragePool{}, errors.New(GetLastError())
	}
	return VirStoragePool{ptr:ptr} , nil
}

/* virtual domain */
func (d *VirDomain) Create() error {
	result := C.virDomainCreate(d.ptr)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}

func (d *VirDomain) Free() error {
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

func (d *VirDomain) Delete() error {
	result := C.virDomainUndefine(d.ptr)
	if result < 0 {
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
	result := C.virDomainGetXMLDesc(d.ptr, C.VIR_DOMAIN_XML_SECURE)
	if result == nil {
		return "", errors.New(GetLastError())
	}
	xml := C.GoString(result)
	C.free(unsafe.Pointer(result))
	return xml, nil
}

type VirStoragePool struct {
	ptr C.virStoragePoolPtr
}

func (c * VirConnection) StoragePoolDefineXML(xml string) (VirStoragePool, error){
	cXML := C.CString(xml)
	defer C.free(unsafe.Pointer(cXML))
	ptr := C.virStoragePoolDefineXML(c.ptr, cXML, 0)
	if ptr == nil {
		return VirStoragePool{}, errors.New(GetLastError())
	}
	return VirStoragePool{ptr:ptr}, nil

}


func (p *VirStoragePool) Create() error {
	result := C.virStoragePoolCreate(p.ptr, 0)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
}


func (p *VirStoragePool) Free() error {
	result := C.virStoragePoolFree(p.ptr)
	if result < 0 {
		return errors.New(GetLastError())
	}
	return nil
}


func (p *VirStoragePool) LookupStorageVolByName(name string) (VirStorageVol, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	ptr := C.virStorageVolLookupByName(p.ptr, cName)
	if ptr == nil {
		return VirStorageVol{}, errors.New(GetLastError())
	}
	return VirStorageVol{ptr: ptr}, nil
}


func (p *VirStoragePool) StorageVolCreateXML(xmlConfig string, flags uint32) (VirStorageVol, error) {
	cXml := C.CString(string(xmlConfig))
	defer C.free(unsafe.Pointer(cXml))
	ptr := C.virStorageVolCreateXML(p.ptr, cXml, C.uint(flags))
	if ptr == nil {
		return VirStorageVol{}, errors.New(GetLastError())
	}
	return VirStorageVol{ptr: ptr}, nil
}


func (p *VirStoragePool) IsActive() bool{
	result := C.virStoragePoolIsActive(p.ptr)
	switch result {
	case -1:
		return false
	case 0:
		return false
	case 1:
		return true
	default:
		return false
	}
}

type VirStorageVol struct {
	ptr C.virStorageVolPtr
}


func (v *VirStorageVol) Free() error {
	if result := C.virStorageVolFree(v.ptr); result != 0 {
		return errors.New(GetLastError())
	}
	return nil
}


func (v *VirStorageVol) GetPath() (string, error) {
	result := C.virStorageVolGetPath(v.ptr)
	defer C.free(unsafe.Pointer(result))
	if result == nil {
		return "", errors.New(GetLastError())
	}
	path := C.GoString(result)
	return path, nil
}


func (v *VirStorageVol) Delete() error {
	//always pass 0
	result := C.virStorageVolDelete(v.ptr, 0)
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}


// event callbacks

func  EventRegisterDefaultImpl() error {
	result := C.virEventRegisterDefaultImpl()
	if result == -1 {
		return errors.New(GetLastError())
	}
	return nil
}

func EventRunDefaultImpl() int {
	result := C.virEventRunDefaultImpl()
	return int(result)
}

type EventHandler interface{
	EventHandle(conn VirConnection, domain VirDomain, event int, detail int)
	FreeHandle()
}


//export EventCallBack
func EventCallBack(cPtr C.virConnectPtr, vPtr C.virDomainPtr, event C.int, detail C.int, cData unsafe.Pointer) {
	var p *EventHandler = (*EventHandler)(cData)
	(*p).EventHandle(VirConnection{ptr:cPtr}, VirDomain{ptr:vPtr}, int(event), int(detail))

}

//export VirFreeCallback
func VirFreeCallback(cData unsafe.Pointer) {
	var p *EventHandler = (*EventHandler)(cData)
	(*p).FreeHandle()
}


func ConnectDomainEventRegister(conn VirConnection,domain VirDomain, eventHandler EventHandler) int {
	if eventHandler == nil {
		fmt.Println("wrong")
		return -1
	}
	r := C.virConnectDomainEventRegisterAny(conn.ptr, domain.ptr, C.VIR_DOMAIN_EVENT_ID_LIFECYCLE,
				C.virConnectDomainEventGenericCallback(C.libvirt_eventcallback_cgo),
				unsafe.Pointer(&eventHandler),
				(C.virFreeCallback)(C.libvirt_virfreecalback_cgo))
	result := int(r)
	return result
}

