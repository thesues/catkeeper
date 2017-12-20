package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"github.com/google/uuid"
	libvirt "github.com/libvirt/libvirt-go"
	_ "github.com/mattn/go-sqlite3"
	"github.com/thesues/catkeeper/utils"
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
	_ "net/url"
	"strconv"
	"sync/atomic"
	"time"
)

// token_id <=> TokenContentForVMInstall
var tokenMap = utils.NewSafeMap()

// domain.name <=> callbackId
var callbackMap = utils.NewSafeMap()

type TokenContentForVMInstall struct {
	Ch        chan string
	Name      string
	Conn      *libvirt.Connect
	IPAddress string
}

//TODO move all sql to model.go
func main() {
	/* init database */
	db, err := sql.Open("sqlite3", "/tmp/post_db.bin")
	if err != nil {
		checkErr(err, "open database failed")
		return
	}
	defer db.Close()

	var scanning int32 = 0

	m := martini.Classic()
	m.Use(render.Renderer())

	m.Get("/", func(r render.Render) {
		r.Redirect("index.html")
	})

	//Install a new VM
	m.Get("/create", func(r render.Render) {
		var Name string
		var IpAddress string

		type HostInfo struct {
			Name      string
			IpAddress string
		}

		type Hosts struct {
			Hosts []HostInfo
		}

		//var hosts []HostInfo
		var hosts Hosts

		rows, err := db.Query("select Name,IpAddress from physicalmachine")
		if err != nil {
			reportError(r, err, "failed to find physicalmachine")
		}
		for rows.Next() {
			rows.Scan(&Name, &IpAddress)
			hosts.Hosts = append(hosts.Hosts, HostInfo{Name, IpAddress})
			//hosts = append(hosts, HostInfo{Name, IpAddress})
		}
		log.Println(hosts)
		r.JSON(200, hosts)
	})

	m.Post("/create", func(r render.Render, req *http.Request) {

		name := req.PostFormValue("Name")
		ip := req.PostFormValue("IpAddress")
		diskSize := req.PostFormValue("disk")
		//check input data
		imageSize, err := strconv.Atoi(diskSize)
		if err != nil {
			reportError(r, err, "convert failed")
			return
		}
		//convert to GB
		imageSize = imageSize << 30

		conn, err := getConnectionFromCacheByIP(ip)
		if err != nil {
			reportError(r, err, "no connections")
			return
		}
		log.Println(conn)

		ch := make(chan string, 100)

		//map token_uuid <=> {channel,name,connection,ip}
		token := uuid.New()
		t := TokenContentForVMInstall{Ch: ch, Name: name, Conn: conn, IPAddress: ip}
		tokenMap.Set(token, t)

		type TokenJson struct {
			Token string
		}
		var tokenJson TokenJson
		tokenJson.Token = token.String()
		r.JSON(200, tokenJson)
	})

	m.Get("/add", func(r render.Render) {
		r.HTML(200, "add", nil)
	})

	//missing validation
	//ADD new PhysicalMachine
	m.Post("/add", func(r render.Render, req *http.Request) {
		var existingname string
		name := req.PostFormValue("Name")
		description := req.PostFormValue("Description")
		IPAddress := req.PostFormValue("IPAddress")
		log.Println(name + description + IPAddress)
		//TODO: add more validations
		err := db.QueryRow("select name from physicalmachine where IpAddress = ?", IPAddress).Scan(&existingname)
		switch {
		case err == sql.ErrNoRows:
			// good to insert the PhysicalMachine
			db.Exec("insert into physicalmachine (Name, IpAddress, Description) values(?,?,?)", name, IPAddress, description)
			r.Redirect("/")
		case err != nil:
			//other error happens
			r.HTML(501, "error", err)
		default:
			//no error happens
			r.HTML(501, "error", fmt.Errorf("already has the physical machine %s", existingname))

		}

	})

	m.Get("/vm/(?P<id>[0-9]+)", func(r render.Render, params martini.Params) {
		id, _ := strconv.Atoi(params["id"])
		vm := getVirtualMachine(db, id)
		r.HTML(200, "vm", vm)
	})

	m.Post("/vm/delete/(?P<id>[0-9]+)", func(r render.Render, params martini.Params) {
		id, _ := strconv.Atoi(params["id"])
		vm := getVirtualMachine(db, id)
		err := vm.Delete(db)
		defer vm.Free()
		if err != nil {
			reportError(r, err, "I can not delete the vm")
		}
		r.HTML(200, "vm", vm)

	})

	m.Post("/rescan", func(req *http.Request) {
		if atomic.LoadInt32(&scanning) == 0 {
			go func() {
				atomic.StoreInt32(&scanning, 1)
				RescanIPAddress(db)
				atomic.StoreInt32(&scanning, 0)

			}()
		}
	})

	m.Post("/vm/(?P<id>[0-9]+)", func(r render.Render, params martini.Params, req *http.Request) {
		req.ParseForm()
		submitType := req.PostFormValue("submit")
		id, _ := strconv.Atoi(params["id"])
		vm := getVirtualMachine(db, id)
		var err error
		switch submitType {
		case "Start":
			err = vm.Start()
			vm.Free()
		case "Stop":
			err = vm.Stop()
			//TODO: wait a while
			vm.Free()
		case "ForceStop":
			err = vm.ForceStop()
			vm.Free()
		case "Update":
			owner := req.PostForm["Owner"][0]
			description := req.PostForm["Description"][0]
			err = vm.UpdateDatabase(db, owner, description)
		}

		if err != nil {
			//error page
			r.HTML(501, "error", err)
			log.Println(err)
		} else {
			//redirect page
			r.Redirect("/")
		}
	})

	//API part
	m.Get("/api/list", func(r render.Render) {
		pm := getListofPhysicalMachineAndVirtualMachine(db)
		r.JSON(200, pm)
	})

	m.Get("/api/vm/(?P<id>[0-9]+)", func(r render.Render, params martini.Params) {
		id, _ := strconv.Atoi(params["id"])
		vm := getVirtualMachine(db, id)
		r.JSON(200, vm)
	})

	vncProxy := websocket.Server{Handler: proxyHandler,
		Handshake: func(ws *websocket.Config, req *http.Request) error {
			ws.Protocol = []string{"base64"}
			return nil
		}}

	m.Get("/websockify", vncProxy.ServeHTTP)

	installState := websocket.Server{Handler: stateReportHandler}

	m.Get("/installsockify", installState.ServeHTTP)

	// rescan IP address every 6 hour
	go func() {
		for {
			time.Sleep(time.Hour * 6)
			if atomic.LoadInt32(&scanning) == 0 {
				log.Println("start planed rescan IP address")
				atomic.StoreInt32(&scanning, 1)
				RescanIPAddress(db)
				atomic.StoreInt32(&scanning, 0)
			}
		}

	}()

	//EventRunDefault
	//this must be before any connection
	libvirt.EventRegisterDefaultImpl()
	go func() {
		for {
			err := libvirt.EventRunDefaultImpl()
			if err != nil {
				fmt.Println("Run failed")
				break
			}
		}
	}()

	//start web server
	http.ListenAndServe(":3000", m)
}

//register an event
func myrebootcallback(c *libvirt.Connect, d *libvirt.Domain, event *libvirt.DomainEventLifecycle) {
	fmt.Printf("Got event %d\n", event.Event)
	if event.Event == libvirt.DOMAIN_EVENT_STOPPED {
		fmt.Println("rebooting...")
		d.Create()
	}
	name, _ := d.GetName()
	if callbackMap.Check(name) == true {
		callbackId := callbackMap.Get(name).(int)
		c.DomainEventDeregister(callbackId)
		callbackMap.Delete(name)
	}
}

func registerRebootAndGetVncPort(name string, ip string, conn libvirt.Connect) string {
	var domain *libvirt.Domain
	domain, err := conn.LookupDomainByName(name)
	if err != nil {
		log.Println("FAIL: find running domain to start vncviewer")
		return ""
	}
	defer domain.Free()

	xmlData, _ := domain.GetXMLDesc(libvirt.DOMAIN_XML_SECURE)
	v := utils.ParseDomainXML(xmlData)

	/* to get VNC port */
	var vncPort string
	if v.Devices.Graphics.VNCPort == "-1" {
		log.Println("FAIL:Can not get vnc port")
		return ""
	}

	vncPort = v.Devices.Graphics.VNCPort

	callbackId, error := conn.DomainEventLifecycleRegister(domain, myrebootcallback)
	if error != nil {
		fmt.Println("can not autoreboot")
	} else {
		callbackMap.Set(name, callbackId)
	}

	vncAddress := ip + ":" + vncPort
	//e.g. http://147.2.207.233/vnc_auto.html?title=lwang-n1-sle12rc1&path=websockify?ip=147.2.207.233:5902

	log.Println(fmt.Sprintf("/vnc_auto.html?title=%s&path=websockify?ip=%s", name, vncAddress))
	return fmt.Sprintf("/vnc_auto.html?title=%s&path=websockify?ip=%s", name, vncAddress)
}

func stateReportHandler(ws *websocket.Conn) {
	log.Println("start installing")
	defer ws.Close()
	r := ws.Request()
	values := r.URL.Query()
	//get token
	_, ok := values["token"]
	if ok == false {
		log.Println("failed to get installation token for url")
		return
	}
	token := values["token"][0]

	if tokenMap.Check(token) == false {
		log.Println("failed to get installation token")
		return
	}

	tokenContent := tokenMap.Get(token).(TokenContentForVMInstall)

	messageChannel := tokenContent.Ch

	//get Domain
	for m := range messageChannel {
		websocket.Message.Send(ws, m)
	}
	tokenMap.Delete(token)
}

func proxyHandler(ws *websocket.Conn) {
	defer ws.Close()
	r := ws.Request()
	values := r.URL.Query()
	ip, hasIp := values["ip"]

	if hasIp == false {
		//log.Println("faile to parse vnc address")
		return
	}

	vc, err := net.Dial("tcp", ip[0])
	if err != nil {
		return
	}
	defer vc.Close()
	done := make(chan bool)

	go func() {
		sbuf := make([]byte, 32*1024)
		dbuf := make([]byte, 32*1024)
		for {
			n, e := ws.Read(sbuf)
			if e != nil {
				done <- true
				return
			}
			n, e = base64.StdEncoding.Decode(dbuf, sbuf[0:n])
			if e != nil {
				done <- true
				return
			}
			n, e = vc.Write(dbuf[0:n])
			if e != nil {
				done <- true
				return
			}
		}
	}()
	go func() {
		sbuf := make([]byte, 32*1024)
		dbuf := make([]byte, 64*1024)
		for {
			n, e := vc.Read(sbuf)
			if e != nil {
				done <- true
				return
			}
			base64.StdEncoding.Encode(dbuf, sbuf[0:n])
			n = ((n + 2) / 3) * 4
			ws.Write(dbuf[0:n])
			if e != nil {
				done <- true
				return
			}
		}
	}()
	select {
	case <-done:
		break
	}
}

func reportError(r render.Render, err error, userError string) {
	r.HTML(501, "error", err.Error()+userError)
}
