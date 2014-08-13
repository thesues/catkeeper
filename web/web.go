package main

import (
	"github.com/codegangsta/martini"
	"github.com/codegangsta/martini-contrib/render"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	_ "net/url"
	"net/http"
	"log"
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"net"
	"strconv"
	"fmt"
	"time"
	"sync/atomic"
	"code.google.com/p/go-uuid/uuid"
	"dmzhang/catkeeper/utils"
)

var tokenMap = utils.NewSafeMap()

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


    m.Get("/", func(r render.Render){
	    r.Redirect("index.html")
    })

    //Install a new VM
    m.Get("/create", func(r render.Render) {
	    var Name string
	    var IpAddress string

	    type HostInfo struct {
		    Name string
		    IpAddress string
	    }

	    var hosts []HostInfo

	    rows, err := db.Query("select Name,IpAddress from physicalmachine")
	    if err != nil {
		    reportError(r, err, "failed to find physicalmachine")
	    }
	    for rows.Next() {
		    rows.Scan(&Name, &IpAddress)
		    hosts = append(hosts, HostInfo{Name, IpAddress})
	    }
	    log.Println(hosts)
	    r.HTML(200, "create", hosts)
    })

    m.Post("/create", func(r render.Render, req *http.Request) {
	    //get req
	    //host
	    //name
            m := CreateVirtualMachine()
	    //generate token
	    token := uuid.New()
	    tokenMap.Set(token,m)
	    redirectStr := fmt.Sprintf("/install.html?token=%s",token)
	    r.Redirect(redirectStr)
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
	    log.Println(name+description+IPAddress)
	    //TODO: add more validations
	    err := db.QueryRow("select name from physicalmachine where IpAddress = ?", IPAddress).Scan(&existingname)
	    switch {
		    case err == sql.ErrNoRows:
			    // good to insert the PhysicalMachine
			    db.Exec("insert into physicalmachine (Name, IpAddress, Description) values(?,?,?)", name, IPAddress,description)
			    r.Redirect("/")
		    case err != nil:
			    //other error happens
			    r.HTML(501,"error", err)
		    default:
			    //no error happens
			    r.HTML(501,"error",fmt.Errorf("already has the physical machine %s", existingname))

	   }

    })

    m.Get("/vm/(?P<id>[0-9]+)", func(r render.Render, params martini.Params){
	    id,_ := strconv.Atoi(params["id"])
	    vm := getVirtualMachine(db,id)
	    r.HTML(200, "vm", vm)
    })


    m.Post("/vm/delete/(?P<id>[0-9]+)", func(r render.Render, params martini.Params) {
	    id,_ := strconv.Atoi(params["id"])
	    vm := getVirtualMachine(db,id)
	    err := vm.Delete(db)
	    if err != nil {
		    reportError(r, err, "I can not delete the vm")
	    }
	    r.Redirect("index.html")
    })

    m.Post("/rescan", func(req *http.Request) {
	    if atomic.LoadInt32(&scanning) == 0{
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
	    id,_ := strconv.Atoi(params["id"])
	    vm := getVirtualMachine(db,id)
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
    m.Get("/api/list", func(r render.Render){
	    pm := getListofPhysicalMachineAndVirtualMachine (db)
	    r.JSON(200, pm)
    })

    m.Get("/api/vm/(?P<id>[0-9]+)", func(r render.Render, params martini.Params){
	    id,_ := strconv.Atoi(params["id"])
	    vm := getVirtualMachine(db,id)
	    r.JSON(200, vm)
    })


    vncProxy := websocket.Server{Handler:proxyHandler,
			    Handshake: func(ws *websocket.Config, req *http.Request) error {
			    ws.Protocol = []string{"base64"}
			    return nil
    }}

    m.Get("/websockify", vncProxy.ServeHTTP)

    installState := websocket.Server{Handler:stateReportHandler}


    m.Get("/installsockify", installState.ServeHTTP)

    // rescan IP address every 6 hour
    go func(){
	    for {
		    time.Sleep(time.Hour * 6)
		    if atomic.LoadInt32(&scanning) == 0{
			    log.Println("start planed rescan IP address")
			    atomic.StoreInt32(&scanning, 1)
			    RescanIPAddress(db)
			    atomic.StoreInt32(&scanning, 0)
		    }
	    }

    }()

    m.Run()
}

func stateReportHandler(ws *websocket.Conn) {
	log.Println("start installing")
	defer ws.Close()
	r := ws.Request()
	values := r.URL.Query()
	//get token
	_ , ok := values["token"]
	if ok == false {
		log.Println("failed to get installation token for url")
		return
	}
	token := values["token"][0]

	if tokenMap.Check(token) == false {
		log.Println("failed to get installation token")
		return
	}

	messageChannel := tokenMap.Get(token).(chan string)

	defer close(messageChannel)
	defer tokenMap.Delete(token)

	for m := range messageChannel{
		//output some data
		log.Println(m)
		websocket.Message.Send(ws, m)
	}
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

func CreateVirtualMachine() chan string {
	m := make(chan string, 100)
	/* TODO add the real part */
	go func() {
		m <- "Start"
		time.Sleep( 3 * time.Second)
		m <- "Upload"
		time.Sleep( 3 * time.Second)
		m <- "Sleeping"
		time.Sleep( 10 * time.Second)
		m <- "CLOSE"
		m <- "INFORMATION"
	}()
	return m
}

func reportError(r render.Render,err error, userError string) {
	r.HTML(501,"error", err.Error() + userError)
}

