package vminstall
import (
	"text/template"
	"log"
	"dmzhang/catkeeper/libvirt"
	"bytes"
	"errors"
	"regexp"
)


const (
	OSSECTION = "<os> <type arch='x86_64'>hvm</type><boot dev='hd'/></os>"
	ONBOOT = "<on_reboot>restart</on_reboot>"
	ONCRASH ="<on_crash>restart</on_crash>"
	POOXML = `
	<pool type='dir'>
	<name>boot-scratch</name>
	<target>
	<path>/var/lib/libvirt/boot</path>
	</target>
	</pool>
	`
	DOMAINXML = `

	<domain type='kvm'>
	<name>{{.Name}}</name>
	<memory unit='KiB'>524288</memory>
	<currentMemory unit='KiB'>524288</currentMemory>
	<vcpu placement='static'>4</vcpu>
	<resource>
	<partition>/machine</partition>
	</resource>
	<os>
	<type arch='x86_64' machine='pc-i440fx-1.4'>hvm</type>
	<kernel>{{.Kernel}}</kernel>
	<initrd>{{.Initrd}}</initrd>
	<cmdline>install={{.Install}}</cmdline>
	<boot dev='network'/>
	</os>
	<clock offset='utc'/>
	<on_poweroff>destroy</on_poweroff>
	<on_reboot>destroy</on_reboot>
	<on_crash>destroy</on_crash>
	<devices>
	<emulator>/usr/bin/qemu-kvm</emulator>
	<disk type='file' device='disk'>
	<driver name='qemu' type='qcow2'/>
	<source file='{{.Image}}'/>
	<target dev='vda' bus='virtio'/>
	</disk>
	<graphics type='vnc' autoport='yes'>
	<listen type='address' address='0.0.0.0'/>
	</graphics>
	<interface type='bridge'>
	<source bridge='br2'/>
	<model type='virtio'/>
	</interface>
	</devices>
	<seclabel type='none'/>
	</domain>
	`
	STORAGEXML = `
	<volume>
	<name>{{.Name}}</name>
	<capacity>{{.Size}}</capacity>
	<target>
	<format type='{{.Type}}'/>
	</target>
	</volume>
	`
)

// MESSAGE to be sent to the channel
const (
	VMINSTALL_SUCCESS = "SUCCESS"
	VMINSTALL_FAIL    = "FAIL"
)

type  VolumeXMXEncoder interface {
	Encode() (string, error)
}

type Storage struct {
	Name string
	Size uint64
	//StorageType string //file or directory or others
	Type string //raw or qcow2
}

func (v Storage) Encode() (string,error) {
	var t *template.Template
	var err error
	var result bytes.Buffer
	t = template.Must(template.New("storage").Parse(STORAGEXML))
	if err != nil {
		return "",err
	}
	t.Execute(&result, v)
	return result.String(), nil
}

type Domain struct {
	Name string
	Kernel string
	Initrd string
	Image  string
	Install string
}

func (d Domain) Encode() (string,error) {
	var t *template.Template
	var err error
	var result bytes.Buffer
	t = template.Must(template.New("domain").Parse(DOMAINXML))
	if err != nil {
		return "",err
	}
	t.Execute(&result, d)
	return result.String(), nil
}



func createVolume(pool libvirt.VirStoragePool, vol VolumeXMXEncoder) (libvirt.VirStorageVol, error) {

	xml, err := vol.Encode()
	if err != nil {
		return libvirt.VirStorageVol{}, err
	}
	volume, err := pool.StorageVolCreateXML(xml,0)
	if err != nil {
		return libvirt.VirStorageVol{}, err
	}
	return volume,nil
}


func SendLocalToRemote(stream libvirt.VirStream, volume libvirt.VirStorageVol, data []byte) error {

	err := libvirt.StorageVolUpload(volume, stream, 0, uint64(len(data)))
	if err != nil {
		return err
	}
	//transfter volume
	remain := len(data)
	sent := 0
	offset := 0
	DATALEN := 16384
	for remain > 0 {
		if remain > DATALEN {
			sent = stream.Send(data[offset:], DATALEN)
		} else {
			sent = stream.Send(data[offset:], remain)
		}

		if sent < 0 {
			stream.Abort()
			return errors.New("Stream Send return 0")
		}
		if sent == 0 {
			break;
		}
		remain -= sent
		offset += sent

	}
	err = stream.Finish()
	if err != nil {
		return err
	}
	return nil

}

func createRemoteBootPool(conn libvirt.VirConnection) (libvirt.VirStoragePool, error){
        // Test volume pool
        // create vol from pool and Upload
        var pool libvirt.VirStoragePool
	pool, err := conn.StoragePoolLookupByName("boot-scratch")
        if err != nil {
                // pool not existed
                // create on pool named "boot-scrath"
                // TODO
                log.Println("pool not exist")
                //poolXML, _:= ioutil.ReadFile("./pool.xml")
		poolXML := POOXML
                pool, err = conn.StoragePoolDefineXML(string(poolXML))
                if err != nil {
			return libvirt.VirStoragePool{}, err
                }
        }
	return pool,nil
}


func reportStatus(ch chan string, m string) {
	if ch != nil {
		ch <- m
	}
}

func reportFail(ch chan string, info string) {
	if ch != nil {
		ch <- VMINSTALL_FAIL + "|"  + info
	}
}


func reportSuccess(ch chan string) {
	if ch != nil {
		ch <- VMINSTALL_SUCCESS
	}
}

// only support SUSE/x86 for now
// memory is only 512M
// TODO: use uuid to generate new names

func VmInstall(conn libvirt.VirConnection, name string, url string, imageSize uint64, ch chan string) {

	//check input

	if len(name) <= 0 {
		reportFail(ch, "Name too short")
		return
	}

	if imageSize == 0 {
		reportFail(ch, "disk size too short")
	}

	if ch != nil {
		defer close(ch)
	}

	pool, err := createRemoteBootPool(conn)
	defer pool.Free()

	//url := "http://mirror.bej.suse.com/dist/install/SLP/SLE-12-Server-Beta10/x86_64/DVD1"
	linuxSurfix := "/boot/x86_64/loader/linux"
	initrdSurfix := "/boot/x86_64/loader/initrd"

	// Download linux and initrd image from remote 
	reportStatus(ch, "Downloading linux image")
	m := DownloadManager{}
	m.Regsiter(HTTPDownloader{})
	linuxContent, err := m.Download(url+linuxSurfix)
	if err != nil {
		reportFail(ch, err.Error())
		return
	}

	reportStatus(ch, "Downloading initrd image")

	initrdContent, err := m.Download(url+initrdSurfix)
	if err != nil {
		reportFail(ch, err.Error())
		return
	}


	// create remote boot linux storage from temp pool
	linuxVolume, err := createVolume(pool, Storage{Name:"linux-dmzhang", Size:uint64(len(linuxContent)),Type:"raw"})
	if err != nil {
		reportFail(ch, err.Error())
		return
	}
	defer linuxVolume.Free()
	linuxPath, _ := linuxVolume.GetPath()


	// create remote boot initrd storage from temp pool
	initrdVolume, err:= createVolume(pool, Storage{Name:"initrd-dmzhang", Size:uint64(len(initrdContent)), Type:"raw"})
	if err != nil {
		reportFail(ch, err.Error())
		return
	}
	defer initrdVolume.Free()
	initrdPath, _ := initrdVolume.GetPath()



	var stream libvirt.VirStream
	stream, err = conn.StreamNew()
	if err != nil {
		reportFail(ch, err.Error())
		return
	}
	defer stream.Free()

	//Upload to remote
	reportStatus(ch, "sending linuxVolume")
	if err := SendLocalToRemote(stream, linuxVolume, linuxContent); err != nil {
		reportFail(ch, err.Error())
		return
	}


	reportStatus(ch, "sending initrd")
	if err := SendLocalToRemote(stream, initrdVolume, initrdContent); err != nil {
		reportFail(ch, err.Error())
		return
	}


	// create image 
	reportStatus(ch, "creating remote imaging...")
	dataPool, err := conn.StoragePoolLookupByName("default")
	if err != nil {
		reportFail(ch, err.Error())
		return
	}
	defer dataPool.Free()

	//var imageSize uint64 = 8589934592 //8G

	imageVolume,err := createVolume(dataPool, Storage{Name:"linux-dmzhang.img", Size:imageSize, Type:"qcow2"})

	if err != nil {
		reportFail(ch, err.Error())
		return
	}

	defer imageVolume.Free()
	imagePath, _  := imageVolume.GetPath()


	log.Println("Create remote VirtualMachine")
	reportStatus(ch, "Create remote VirtualMachine")

	// create boot xml
	var xml string
	domain := Domain{Name:"sles12beta10_dmzhang", Kernel:linuxPath, Initrd:initrdPath, Image:imagePath, Install:url}
	if xml, err = domain.Encode();err != nil {
		reportFail(ch, err.Error())
		return
	}

	// create booting vm
	bootingDomain, err :=  conn.CreateXML(xml)
	if err != nil {
		reportFail(ch, err.Error())
		return
	}
	defer bootingDomain.Free()


	// get xml from remote
	// create new defined xml
	if xml, err = bootingDomain.GetXMLDesc(); err != nil {
		reportFail(ch, err.Error())
		return
	}

	defer linuxVolume.Delete()
	defer initrdVolume.Delete()

	/* change xml a bit using regex lines, I do not want to parse the xml file
	* 1. change os section to boot from hd
	* 2. change destory section
	*/
	/* (?s) is used to let . match newline(\n) */
	osSection := regexp.MustCompile("(?s)<os>.*</os>")
	onBoot := regexp.MustCompile("(?s)<on_reboot>.*<on_reboot>")
	onCrash := regexp.MustCompile("(?s)<on_crash>.*<on_crash>")
	xml  = osSection.ReplaceAllString(xml, OSSECTION)
	xml = onBoot.ReplaceAllString(xml, ONBOOT)
	xml = onCrash.ReplaceAllString(xml, ONCRASH)


	newPersistentDomain, err := conn.DefineXML(xml)
	if err != nil {
		reportFail(ch, err.Error())
		return
	}

	log.Println(newPersistentDomain)
	defer newPersistentDomain.Free()

	reportSuccess(ch)

}
