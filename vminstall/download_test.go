package vminstall
import (
	"fmt"
	"testing"
	"io/ioutil"
)


func TestDownload(t *testing.T) {
	m := DownloadManager{}
	m.Regsiter(HTTPDownloader{})
	out,err := m.Download("http://www.baidu.com/img/bdlogo.gif")
	if err != nil {
		fmt.Printf("failed to download from %s",nil)
	}
	fmt.Printf("%d download\n", len(out))
	ioutil.WriteFile("bdlogo.gif", out, 0644)
}
