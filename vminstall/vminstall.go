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
	<driver name='qemu' type='raw'/>
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
	<volume type='{{.StorageType}}'>
	<name>{{.Name}}</name>
	<capacity>{{.Size}}</capacity>
	<target>
	<format type='{{.Type}}'/>
	</target>
	</volume>
	`
)

type  VolumeXMXEncoder interface {
	Encode() (string, error)
}

type Storage struct {
	Name string
	Size uint64
	StorageType string //file or directory or others
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
	log.Println("Finish Send Data...")
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

// only support SUSE/x86 for now
// memory is only 512M
// TODO: use uuid to generate new names

func VmInstall(conn libvirt.VirConnection, name string, url string, imageSize uint64, ch chan string) error {

	//check input

	if len(name) <= 0 {
		return errors.New("Name too short")
	}

	if imageSize == 0 {
		return errors.New("disk size to short")
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
	m := DownloadManager{}
	m.Regsiter(HTTPDownloader{})
	linuxContent, err := m.Download(url+linuxSurfix)

	log.Println("Downloading linux image")
	reportStatus(ch, "Downloading linux image")

	if err != nil {
		log.Println("download linux image failed")
		return err
	}

	log.Println("Downloading initrd image")
	reportStatus(ch, "Downloading initrd image")

	initrdContent, err := m.Download(url+initrdSurfix)
	if err != nil {
		log.Println("download initrd image failed")
		return err
	}


	// create remote boot linux storage from temp pool
	linuxVolume, err := createVolume(pool, Storage{Name:"linux-dmzhang", Size:uint64(len(linuxContent)), StorageType:"file", Type:"raw"})
	if err != nil {
		log.Println("can not Create Volume")
		return err
	}
	defer linuxVolume.Free()
	linuxPath, _ := linuxVolume.GetPath()


	// create remote boot initrd storage from temp pool
	initrdVolume, err:= createVolume(pool, Storage{Name:"initrd-dmzhang", Size:uint64(len(initrdContent)), StorageType:"file", Type:"raw"})
	if err != nil {
		log.Println("can not Create Volume")
		return err
	}
	defer initrdVolume.Free()
	initrdPath, _ := initrdVolume.GetPath()



	var stream libvirt.VirStream
	stream, err = conn.StreamNew()
	if err != nil {
		log.Printf("failed to create stream")
		return err
	}
	defer stream.Free()

	//Upload to remote
	log.Println("sending linuxVolume")
	reportStatus(ch, "sending linuxVolume")

	if err := SendLocalToRemote(stream, linuxVolume, linuxContent); err != nil {
		log.Println(err)
		return err
	}

	log.Println("sending initrd")
	reportStatus(ch, "sending initrd")

	if err := SendLocalToRemote(stream, initrdVolume, initrdContent); err != nil {
		log.Println(err)
		return err
	}


	// create image 
	log.Println("create remote image")
	reportStatus(ch, "creating remote imaging...")

	dataPool, err := conn.StoragePoolLookupByName("default")
	if err != nil {
		log.Println("can not get default pool")
		return err
	}
	defer dataPool.Free()

	//var imageSize uint64 = 8589934592 //8G

	imageVolume,err := createVolume(dataPool, Storage{Name:"linux-dmzhang.img", Size:imageSize, StorageType:"file", Type:"raw"})
	if err != nil {
		log.Println("WHAT?!!")
		return err
	}
	defer imageVolume.Free()
	imagePath, _  := imageVolume.GetPath()


	log.Println("Create remote VirtualMachine")
	reportStatus(ch, "Create remote VirtualMachine")

	// create boot xml
	var xml string
	domain := Domain{Name:"sles12beta10_dmzhang", Kernel:linuxPath, Initrd:initrdPath, Image:imagePath, Install:url}
	if xml, err = domain.Encode();err != nil {
		log.Printf("encode domain failed %s",err)
		return err
	}

	// create booting vm
	bootingDomain, err :=  conn.CreateXML(xml)
	if err != nil {
		log.Printf("failed to create remote booting vm")
		return err
	}
	defer bootingDomain.Free()


	// get xml from remote
	// create new defined xml
	if xml, err = bootingDomain.GetXMLDesc(); err != nil {
		log.Printf("can not get xml from remote booting vm")
		return err
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
		log.Println(err)
		return err
	}
	log.Println(newPersistentDomain)

	reportStatus(ch, "Finish Create the Virtual Machine")

	//close the report channel
	// no error happend
	return nil
}
