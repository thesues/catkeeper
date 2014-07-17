package main

import (
	"fmt"
	"testing"
	"encoding/xml"
)

const xmlData = `
<domain type='kvm' id='4'>
  <name>cl8_n1_sles12b8</name>
  <uuid>016db229-a046-26a8-3956-85c7fca5f969</uuid>
  <memory unit='KiB'>1048576</memory>
  <currentMemory unit='KiB'>1048576</currentMemory>
  <vcpu placement='static'>1</vcpu>
  <resource>
    <partition>/machine</partition>
  </resource>
  <os>
    <type arch='x86_64' machine='pc-i440fx-1.4'>hvm</type>
    <boot dev='hd'/>
  </os>
  <features>
    <acpi/>
    <pae/>
  </features>
  <clock offset='utc'/>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <devices>
    <emulator>/usr/bin/qemu-kvm</emulator>
    <disk type='file' device='disk'>
      <driver name='qemu' type='qcow2'/>
      <source file='/mnt/vm/cl8_n1_sles12b8/disk0.qcow2'/>
      <target dev='hda' bus='ide'/>
      <alias name='ide0-0-0'/>
      <address type='drive' controller='0' bus='0' target='0' unit='0'/>
    </disk>
    <controller type='usb' index='0'>
      <alias name='usb0'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x2'/>
    </controller>
    <controller type='ide' index='0'>
      <alias name='ide0'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x1'/>
    </controller>
    <controller type='pci' index='0' model='pci-root'>
      <alias name='pci0'/>
    </controller>
    <interface type='bridge'>
      <mac address='52:54:00:6f:d6:f1'/>
      <source bridge='br1'/>
      <target dev='vnet2'/>
      <model type='rtl8139'/>
      <alias name='net0'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x03' function='0x0'/>
    </interface>
    <interface type='bridge'>
      <mac address='52:54:00:82:d4:da'/>
      <source bridge='br1'/>
      <target dev='vnet3'/>
      <model type='rtl8139'/>
      <alias name='net1'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x05' function='0x0'/>
    </interface>
    <input type='mouse' bus='ps2'/>
    <graphics type='vnc' port='5902' autoport='yes' listen='0.0.0.0'>
      <listen type='address' address='0.0.0.0'/>
    </graphics>
    <video>
      <model type='cirrus' vram='9216' heads='1'/>
      <alias name='video0'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0'/>
    </video>
    <memballoon model='virtio'>
      <alias name='balloon0'/>
      <address type='pci' domain='0x0000' bus='0x00' slot='0x04' function='0x0'/>
    </memballoon>
  </devices>
  <seclabel type='none'/>
</domain>
`

func TestParse(t *testing.T) {
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

	xml.Unmarshal([]byte(xmlData),&v)

	if v.Name != "cl8_n1_sles12b8" {
		t.Errorf("parse Name failed")
	}

	if v.Devices.Graphics.VNCPort != "5902" {
		t.Errorf("parse VNCport failed")
	}

	var mac_address = make(map[string]string)

	for _, i := range v.Devices.Interface {
		if i.Type == "bridge" {
			mac_address[i.MAC.Address] = "not detected"
		}
	}
	fmt.Println(mac_address)
}

