package utils
import (
	"encoding/xml"
)


type VNCinfo struct {
	VNCPort string `xml:"port,attr"`
}

type MACAttr struct {
	Address string `xml:"address,attr"`
}
type BridgeInterface struct {
	MAC MACAttr`xml:"mac"`
	Type string `xml:"type,attr"`

}

type DiskSource struct {
	Path string `xml:"file,attr"`
}
type Disk struct {
	Source DiskSource `xml:"source"`
}

type Devices struct {
	Graphics VNCinfo `xml:"graphics"`
	Interface []BridgeInterface `xml:"interface""`
	Disks []Disk `xml:"disk"`
}

type xmlParseResult struct {
	Name string    `xml:"name"`
	UUID string    `xml:"uuid"`
	Devices  Devices `xml:"devices"`
}

func ParseDomainXML(xmlData string) *xmlParseResult {
	var v = xmlParseResult{}
	xml.Unmarshal([]byte(xmlData),&v)
	return &v
}
