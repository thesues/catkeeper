package vminstall

import (
	"errors"
	"net/http"
	"io/ioutil"
	"strings"
)

type Downloader interface{
	Download(url string) ([]byte, error)
	Match() string
}

type HTTPDownloader struct {

}

func (h HTTPDownloader) Match() string {
	return "http"
}

func (h HTTPDownloader) Download(url string) ([]byte,error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return contents,nil

}

type DownloadManager struct {
	downloaders []Downloader
}


func (manager *DownloadManager) Regsiter(d Downloader) {
	manager.downloaders = append(manager.downloaders, d)
}


func (manager *DownloadManager) Download(url string) ([]byte,error) {
	//find an good downloader
	var found bool = false
	var d Downloader
	for _,d = range manager.downloaders {
		if strings.Index(url, d.Match()) == 0 {
			found = true
			break
		}
	}

	if found {
		return d.Download(url)
	} else {
		return nil, errors.New("not found matching download")
	}

}
