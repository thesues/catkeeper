package libvirt


/*
#include <libvirt/libvirt.h>
void libvirt_eventcallback_cgo(virConnectPtr c, virDomainPtr d, int event, int detail, void * data) {
	EventCallBack(c, d, event, detail, data);
}

void libvirt_virfreecalback_cgo(void *opaque){
	VirFreeCallback(opaque);
}
*/
import "C"
