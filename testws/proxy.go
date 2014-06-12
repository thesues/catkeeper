package main

import (
	"code.google.com/p/go.net/websocket"
	"encoding/base64"
	"flag"
	"log"
	"net"
	"net/http"
)

var (
	listen = flag.String("listen", ":6080", "Location to listen for connections")
)

func main() {
	flag.Parse()
	var wsConfig *websocket.Config
	var err error
	if wsConfig, err = websocket.NewConfig("ws://127.0.0.1:6080/", "http://127.0.0.1:6080"); err != nil {
		log.Fatalf(err.Error())
		return
	}

	// wsConfig.Protocol = []string{"base64"}
	http.Handle("/websockify", websocket.Server{Handler: wsh,
		Config: *wsConfig,
		Handshake: func(ws *websocket.Config, req *http.Request) error {
			ws.Protocol = []string{"base64"}
			return nil
		}})
	http.Handle("/novnc/", http.StripPrefix("/novnc/", http.FileServer(http.Dir("./novnc/"))))
	log.Fatal(http.ListenAndServe(*listen, nil))
}

func wsh(ws *websocket.Conn) {
	loc := "147.2.207.233:5901"
	vc, err := net.Dial("tcp", loc)
	defer vc.Close()
	if err != nil {
		log.Print(err)
		return
	}
	go func() {
		sbuf := make([]byte, 32*1024)
		dbuf := make([]byte, 32*1024)
		for {
			n, e := ws.Read(sbuf)
			if e != nil {
				return
			}
			n, e = base64.StdEncoding.Decode(dbuf, sbuf[0:n])
			if e != nil {
				return
			}
			n, e = vc.Write(dbuf[0:n])
			if e != nil {
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
				return
			}
			base64.StdEncoding.Encode(dbuf, sbuf[0:n])
			n = ((n + 2) / 3) * 4
			ws.Write(dbuf[0:n])
			if e != nil {
				return
			}
		}
	}()
	select {}
}

