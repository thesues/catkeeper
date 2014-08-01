package libvirt


/*
#include <libvirt/libvirt.h>
#include <stdio.h>
void libvirt_eventcallback_cgo(virConnectPtr c, virDomainPtr d, int event, int detail, void * data) {
	printf("good");
	EventCallBack(c, d, data);
}

void libvirt_virfreecalback_cgo(void *opaque){
	VirFreeCallback(opaque);
}
*/
import "C"
