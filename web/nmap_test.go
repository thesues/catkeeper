package main
import (
	"testing"
	"fmt"
)


func TestNmap(t *testing.T) {
	// TODO find my own network 
	fmt.Println(Nmap("147.2.212.0/24"))
}
