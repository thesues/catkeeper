package main
import (
	"testing"
	"fmt"
)


func TestNmap(t *testing.T) {
	version, err := CheckNmapVersion()
	var o map[string][]string
	var args []string

	switch version {
	case -1:
		fmt.Println(err)
		return
	case 4:
		args = []string{"-n", "-sP", "147.2.212.0/24"}
		o,_ = Nmap(args, ParseNmapOutput475)
	case 6:
		args := []string{"-sn", "-n", "147.2.212.0/24"}
		o,_ = Nmap(args, ParseNmapOutput640)
	}
	fmt.Println(o)
}
