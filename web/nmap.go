package main

import ("os"
	"os/exec"
	"regexp"
	"strings"
	"errors"
	"strconv"
)


type Parser func (string) map[string][]string


func CheckNmapVersion() (int, error){
	if os.Geteuid() != 0 {
		return -1, errors.New("must be run by root or you can not get MAC address from nmap")
	}
	version := -1
	_, err := exec.LookPath("nmap")
	if err != nil {
		return -1, errors.New("failed to find nmap")
	}

	cmd := exec.Command("nmap", "-v")

	out, err := cmd.Output()
	if err != nil {
		return -1, errors.New("excuting nmap failed")
	}

	versionPattern := regexp.MustCompile(`Starting Nmap ([0-9])\.`)
	if obj := versionPattern.FindStringSubmatch(string(out));len(obj) > 1 {
		if version,err = strconv.Atoi(obj[1]);err != nil {
			return -1, errors.New("failed to find version")
		}
		return version, nil
	} else {
		return -1, errors.New("failed to find version")
	}

}

func Nmap(args []string, p Parser) (map[string][]string, error) {
	cmd := exec.Command("nmap", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.New("excuting nmap failed")
	}

	return p(string(out)), nil
}


func ParseNmapOutput475(lines string) map[string][]string{
	var ip string  = ""
	HwIpDict := make(map[string][]string)
	ipPattern := regexp.MustCompile(`Host ([0-9.]+) appears to be up`)
	hwPattern := regexp.MustCompile(`MAC Address: ([0-9A-Z:]+) \(QEMU Virtual NIC\)`)

	for _,line := range strings.Split(lines, "\n") {
		if obj := ipPattern.FindStringSubmatch(line); len(obj) > 1 {
			ip = obj[1]
		} else if ip != ""  {
			if obj :=hwPattern.FindStringSubmatch(line); len(obj) > 1 {
				HwIpDict[obj[1]] = append(HwIpDict[obj[1]], ip)
				ip = ""
			}
		}
	}
	return HwIpDict
}

/*
Nmap scan report for 147.2.212.57
Host is up (0.00077s latency).
MAC Address: 00:1E:C9:47:63:DF (Dell)
*/
func ParseNmapOutput640(lines string) map[string][]string {
	var ip string = ""
	HwIpDict := make(map[string][]string)
	ipPattern := regexp.MustCompile(`Nmap scan report for ([0-9.]+)`)
	hwPattern := regexp.MustCompile(`MAC Address: ([0-9A-Z:]+).*`)
	for _,line := range strings.Split(lines, "\n") {
		if obj := ipPattern.FindStringSubmatch(line); len(obj) > 1 {
			ip = obj[1]
		} else if ip != ""  {
			if obj :=hwPattern.FindStringSubmatch(line); len(obj) > 1 {
				HwIpDict[obj[1]] = append(HwIpDict[obj[1]], ip)
				ip = ""
			}
		}
	}
	return HwIpDict

}
