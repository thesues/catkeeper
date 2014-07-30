package main
import (
	"dmzhang/catkeeper/libvirt"
	"dmzhang/catkeeper/vminstall"
	"fmt"
	"flag"
	"os/exec"
	"encoding/xml"
)

func usage() {
	/*
	fmt.Println("USAGE:")
	fmt.Println("vminstallbin -name=<NAME> -repo=<REPOSITORY URL> -host=<PHYSICAL HOST IPADDRESS> -size= <num>(UNIT G), -autoyast=<HTTP://XMLLOCATION>")
	*/
	fmt.Println("\nEXAMPLE:")
	fmt.Println("vminstallbin -host=147.2.207.234 -name=my_test -repo=http://147.2.207.240/repo/SLP/sles10/")
	fmt.Println("vminstallbin -host=147.2.207.234 -name=my_test -repo=http://147.2.207.240/repo/SLP/sles10/ --autoyast=http://147.2.207.233/ay/autoinst.xml")

}

func main() {

	var (
		hostPtr = flag.String("host", "", "remote host IP address")
		repoPtr = flag.String("repo", "", "installation repository")
		autoyastPtr = flag.String("autoyast", "", "location of autoyast xml")
		imageSizePtr = flag.Uint64("size", 10, "image size (G)")
		namePtr = flag.String("name", "", "name of the Virtual Machine")
	)

	flag.Parse()

	var remoteURL string
	if *hostPtr == "" {
		remoteURL = "qemu+ssh:///system"
	} else {
		remoteURL = "qemu+ssh://root@" + *hostPtr+ "/system"
	}

	if *repoPtr == "" {
		fmt.Println("MISSING repo")
		usage()
		return
	}
	repo := *repoPtr

	if *namePtr == "" {
		fmt.Println("MISSING name")
		usage()
		return
	}
	name := *namePtr

	if *autoyastPtr == "" {
		fmt.Println("You did not have autoyast.xml")
	}
	autoinst := *autoyastPtr

	// GB > Byte
	imageSize := *imageSizePtr << 30


	fmt.Printf("Install From :%s \n" , remoteURL)
	fmt.Printf("Name         :%s \n" , name)
	fmt.Printf("Disk Size    :%dG\n", *imageSizePtr)
	fmt.Printf("Repository   :%s \n" , repo)
	fmt.Printf("AutoYast     :%s \n" , autoinst)


	// create remote pool
	fmt.Printf("Creating connection to %s\n", *hostPtr)
	conn, err := libvirt.NewVirConnection(remoteURL)
	if (err != nil) {
		fmt.Println(err)
		return
	}
	defer conn.CloseConnection()

	ch := make(chan string)


	go vminstall.VmInstall(conn, name, repo, autoinst, uint64(imageSize), ch)

	for m := range ch {
		if m == vminstall.VMINSTALL_SUCCESS {
			startVNCviewer(conn, name, *hostPtr)
		} else {
			fmt.Println(m)
		}
	}
}


func startVNCviewer(conn libvirt.VirConnection, name string, hostIPAddress string) {
	fmt.Println("would bring up vncviewer...")
	var domain libvirt.VirDomain
	domain ,err := conn.LookupByName(name)
	if err != nil {
		fmt.Println("FAIL: find running domain to start vncviewer")
		return
	}

	/* FIXME XML parse and safe-map should has own package */
	type MACAttr struct {
		Address string `xml:"address,attr"`
	}
	type BridgeInterface struct {
		MAC MACAttr`xml:"mac"`
		Type string `xml:"type,attr"`

	}
	type VNCinfo struct {
		VNCPort string `xml:"port,attr"`
	}
	type Devices struct {
		Graphics VNCinfo `xml:"graphics"`
		Interface []BridgeInterface `xml:"interface""`
	}

	type xmlParseResult struct {
		Name string    `xml:"name"`
		UUID string    `xml:"uuid"`
		Devices  Devices `xml:"devices"`
	}

	v := xmlParseResult{}
	/* FIXME:code copied from web */
	xmlData, _ := domain.GetXMLDesc()
	xml.Unmarshal([]byte(xmlData), &v)

	/* to get VNC port */
	var vncPort string
	if (v.Devices.Graphics.VNCPort == "-1") {
		fmt.Println("FAIL:Can not get vnc port")
		return
	}
	vncPort =  v.Devices.Graphics.VNCPort
	fmt.Println("RUNNING: vncviewer " + hostIPAddress + ":" + vncPort)
	cmd := exec.Command("vncviewer", hostIPAddress + ":" + vncPort)
	err = cmd.Start()
	if err != nil {
		fmt.Println("FAIL:can not start vncviewer")
		fmt.Println(err)
		return
	}

}
