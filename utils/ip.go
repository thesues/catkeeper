package utils
import (
	"net"
)
func LocalIPs() []string{

        localNet := make([]string,0)
        x,_ := net.InterfaceAddrs()
        for _,i := range x {
                p,ok := i.(*net.IPNet)
                if !ok {
                        continue
                }
                v4 := p.IP.To4()
                if v4 == nil || v4[0] == 127 { // the loopback address
                        continue
                }
                localNet = append(localNet,p.String())
        }
        return  localNet
}

