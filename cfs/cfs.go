package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/gorilla/websocket"
)

func errorResponse(rid, msg string) *map[string]interface{} {
	return &map[string]interface{}{"error": msg, "rid": rid}
}

func fileOperation(v *LocalVolume, conn *websocket.Conn) {
	for {
		var op map[string]json.Number
		err := conn.ReadJSON(&op)
		if err != nil {
			return
		}
		log.Println("op:", op)
		rid := op["rid"].String()
		switch op["op"].String() {
		case "stat":
			st, err := v.Stat(op["path"].String())
			if err != nil {
				conn.WriteJSON(errorResponse(rid, "readdir error"))
				continue
			}
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "stat": st})
		case "read":
			l, _ := op["l"].Int64()
			p, _ := op["p"].Int64()
			b := make([]byte, l)
			len, _ := v.Read(op["path"].String(), b, p)
			conn.WriteMessage(websocket.BinaryMessage, b[:len])
		case "write":
			p, _ := op["p"].Int64()
			b := []byte(op["b"].String())
			len, _ := v.Write(op["path"].String(), b, p)
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "l": len})
		case "remove":
			_ = v.Remove(op["path"].String())
			conn.WriteJSON(&map[string]interface{}{"rid": rid})
		case "files":
			files, err := v.ReadDir(op["path"].String())
			if err != nil {
				conn.WriteJSON(errorResponse(rid, "readdir error"))
				continue
			}
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "files": files})
		default:
			conn.WriteJSON(errorResponse(rid, "unknown operation"))
		}
	}
}

func onConnect(v *LocalVolume, conn *websocket.Conn, target string) {
	log.Println("connect", target)
	var event map[string]string
	if target == "file" {
		fileOperation(v, conn)
	} else {
		err := conn.ReadJSON(&event)
		if err != nil {
			return
		}
		log.Println(event)
	}
	log.Println("disconnect")
}

func publish(localPath, volumePath string, writable bool) {
	fmt.Println("publish ", localPath, " to ", volumePath)

	v := NewLocalVolume(localPath, volumePath, writable)

	// proxy mode
	wsConn, err := ConnectVolume(volumePath, "dummysecret")
	if err != nil {
		log.Fatalln(err)
	}
	defer wsConn.Close()
	finish := make(chan int)
	wsConn.WriteJSON(&map[string]string{"action": "volume", "name": "fuga", "url": "ws://localhost:8080/"})

	go func() {
		// listen loop
		for {
			var event map[string]string
			err := wsConn.ReadJSON(&event)
			if err != nil {
				break
			}
			log.Println(event)
			if event["action"] == "connect" {
				newConn, _ := Connect(event["ws_url"], "", "")
				go onConnect(v, newConn, event["target"])
			}
		}
		finish <- 1
	}()
	fmt.Println("wait...")
	<-finish
	fmt.Println("finished.")
}

func mount(volumePath, mountPoint string) {
	conn, err := ConnectViaPloxy(volumePath, "")
	if err != nil {
		log.Println(err)
	}
	var data = map[string]string{}
	conn.ReadJSON(data) // wait to establish.

	v := NewRemoteVolume(volumePath, conn)
	v.Start()

	files, err := v.ReadDir("")
	log.Println("Readdir", files, err)

	fuseMount(v, mountPoint)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func mount0(volumePath, mountPoint string) {
	fuseMount(nil, mountPoint)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func usage() {
	log.Printf("usage: cs help [command]")
	log.Printf("       cs publish local user/volume")
	log.Printf("       cs mount user/volume mountpoint")
}

func main() {
	writable := flag.Bool("w", false, "writable")
	flag.Parse()
	if flag.Arg(0) == "help" || flag.NArg() == 0 {
		usage()
		return
	}
	if flag.Arg(0) == "publish" && flag.NArg() >= 3 {
		publish(flag.Arg(1), flag.Arg(2), *writable)
	}
	if flag.Arg(0) == "mount" && flag.NArg() >= 3 {
		mount(flag.Arg(1), flag.Arg(2))
	}
}
