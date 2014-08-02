# catkeeper

A virt-manager-like web application

Why:

features:

- Create/Stop/Update virtual machine information
- additional information such as who reserved this
- web VNC support
- automatically scan virtual machine IP address
- filter virtual machine
- should be faster than python/ruby


# Go

1. native language;type between static language and dynamic language
2. goroutine
   [coroutines](http://www.chiark.greenend.org.uk/~sgtatham/coroutines.html)
3. very easy to write network application
4. rich libary support
5. cgo
6. Applications:
   docker

# libvirt Bindings

call C code from Go

	package print

	// #include <stdio.h>
	// #include <stdlib.h>
	import "C"
	import "unsafe"

	func Print(s string) {
	    cs := C.CString(s)
	    defer C.free(unsafe.Pointer(cs))
	    C.fputs(cs, (*C.FILE)(C.stdout))
	}

call Go code from C

callback functions

	https://code.google.com/p/go-wiki/wiki/cgo

# libvirt

## Connection

example:

qemu+ssh://root@147.2.207.233/system
  
qemu: one of the underlining drivers(xen,lxc,)

ssh: conntection method(tls)

## ssh tunnel

background:

vncviewer 147.2.207.233:5901

nc newsmth.net 23


code from virt-viewer

	char *cmd[10] = {"ssh", "-p", "22", "nc", "147.2.207.233","5901"}
	virt_viewer_app_open_tunnel(const char **cmd)
	{
	    int fd[2];
	    pid_t pid;

	    if (socketpair(PF_UNIX, SOCK_STREAM, 0, fd) < 0)
		return -1;

	    pid = fork();
	    if (pid == -1) {
		close(fd[0]);
		close(fd[1]);
		return -1;
	    }

	    if (pid == 0) { /* child */
		close(fd[0]);
		close(0);
		close(1);
		if (dup(fd[1]) < 0)
		    _exit(1);
		if (dup(fd[1]) < 0)
		    _exit(1);
		close(fd[1]);
		execvp("ssh", (char *const*)cmd);
		_exit(1);
	    }
	    close(fd[1]);
	    return fd[0];
	}
	

## Do something 

  two methods to manipulate VM

  1. direct manipulate by RPC API
     Stop/Start/Destory
  2. EDIT XML by RPC API

  A virtual machine = metadata(xml) + data(disk image)
  So the metadata(XML) is very important

     Create Domain/Storage/StoragePool with XML
     e.g create new virtual machine
     1. Define a closed VM using XML
     2. Create VM(Start VM)

     e.g. change VM's Name
     1. Get Domain's XML
     2. edit origin XML into new XML 
     3. Undefine VM
     4. Define VM 
     4. Create VM

# Install REMOTE VM through http repo and autoyast

1. Download initrd/linux from http repo
2. create two file in the remote node by 
3. Upload initrd, kernel (libvirt stream) 
4. create disk file for VM
   4.1 could be any Storage Type(logical volume, file, iscsi)
6. generate BOOT.XML
7. generate FINAL.XML
8. difference bewteen BOOT.XML and FINAL.XML
9. Bootup a temperary VM(do not write the xml to disk) using BOOT.XML
10. Define a persistent VM(do not start it) using FINAL.XML
11. monitor the reboot events of VM
    if rebooting, start the persistent VM

# take a break
  1. vminstall and virt-install
  2. if you close virt-manage before installinig is finished, the vm can not be rebooted

# go web framework

[martini](http://martini.codegangsta.io/)
pros:

1. routing
2. JSON rending

cons:

1. I have to write SQL myself

## Get VM INFO

1. lookup database for all the physicall machine
2. lookup libvirt for all information
   if Connection is cached and is alive:
	get Information (parsing XML)
   else:
	re-connect physicall machine
	put new connection into cache
3. lookup database again for details of virtual machines

goroutine could be used here to get VM information 


## Edit VM INFO

1. Get VM INFO
2. edit Vm throught libvirt
3. edit database

## noVNC

1. noVNC is used in openstack
2. server uses websocket to push data
3. HTML5 to render the screen
4. qemu in sle11sp3 does not support websocket

web browser <=> catkeeper proxy  <=>  qemu

## Install VM

1. only a binary is available

	vminstallbin --host= 147.2.207.133 --repo=http://147.2.207.233/repo/SLP/sle12-beta10/x86\_64/DVD1 --disk=8 --autoyast=http://a.xml

## Scan IP

1. arp -an 
2. #nmap -sP net


## How to filter

javascript get json data from web server

# What I learned?

1. SSH TUNNEL in C
2. libvirt API
3. Python is easier to write
