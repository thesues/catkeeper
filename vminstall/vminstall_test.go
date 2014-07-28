package vminstall
import (
	"dmzhang/catkeeper/libvirt"
	"fmt"
	"testing"
)




func TestVmInstall(t *testing.T) {
	// create remote pool
	fmt.Println("Creating connection")
	conn, err := libvirt.NewVirConnection("qemu+ssh://root@147.2.207.233/system")
	if (err != nil) {
		fmt.Println(err)
		return
	}
	defer conn.CloseConnection()

	ch := make(chan string)

	url := "http://mirror.bej.suse.com/dist/install/SLP/SLE-12-Server-Beta10/x86_64/DVD1"
	name := "dmzhang-test-lifecyle"
	imageSize := 8589934592
	go VmInstall(conn, name, url, uint64(imageSize), ch)

	for m := range ch {
		fmt.Println(m)
	}
}

