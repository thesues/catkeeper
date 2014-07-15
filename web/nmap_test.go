package main
import (
	"testing"
	"fmt"
)


func TestNmap(t *testing.T) {
	// TODO find my own network 
	for _,ip := range LocalIPs() {
		fmt.Println(Nmap(ip))
	}
}
