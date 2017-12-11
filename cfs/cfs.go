package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

var DefaultHubAPI = "http://localhost:8080"
var DefaultHubToken = "dummysecret"

func hubURL() string {
	Url := os.Getenv("CFS_HUB_URL")
	if Url == "" {
		Url = DefaultHubAPI
	}
	return Url
}

func hubToken() string {
	Token := os.Getenv("CFS_HUB_TOKEN")
	if Token == "" {
		Token = DefaultHubToken
	}
	return Token
}

func errorResponse(rid, msg string) *map[string]interface{} {
	return &map[string]interface{}{"error": msg, "rid": rid}
}

func fileOperation(v Volume, conn *websocket.Conn) {
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

func onConnect(v Volume, conn *websocket.Conn, target string) {
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

func publish(localPath, volumePath string, writable bool) error {
	log.Println("publish ", localPath, " to ", volumePath)

	wsConn, err := ConnectVolume(volumePath, hubToken())
	if err != nil {
		log.Println(err)
		return err
	}
	defer wsConn.Close()
	finish := make(chan int)
	wsConn.WriteJSON(&map[string]string{"action": "volume", "name": "fuga", "url": "ws://localhost:8080/"})

	v := NewLocalVolume(localPath, volumePath, writable)

	go func() {
		// listen loop
		for {
			var event map[string]string
			err := wsConn.ReadJSON(&event)
			if err != nil {
				log.Println("read error: ", err)
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

	log.Println("wait...")
	<-finish
	log.Println("finished.")
	return nil
}

func mount(volumePath, mountPoint string) error {
	log.Println("mount ", volumePath, " to ", mountPoint)

	conn, err := ConnectViaPloxy(volumePath, "")
	if err != nil {
		log.Println(err)
		return err
	}
	defer conn.Close()

	v := NewRemoteVolume(volumePath, conn)
	v.Start()

	fuseMount(v, mountPoint)
	log.Println("finished.")
	return nil
}

func usage() {
	log.Printf("usage: cs help [command]")
	log.Printf("       cs publish local user/volume")
	log.Printf("       cs mount user/volume mountpoint")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	writable := fs.Bool("w", false, "writable")
	fs.Parse(os.Args[2:])

	cmd := os.Args[1]
	switch {
	case cmd == "help":
		usage()
	case cmd == "publish" && fs.NArg() >= 3:
		for {
			// TODO: fix retry loop
			publish(fs.Arg(0), fs.Arg(1), *writable)
			time.Sleep(time.Second * 5)
		}
	case cmd == "publish" && fs.NArg() >= 2:
		publish(fs.Arg(0), fs.Arg(1), *writable)
	case cmd == "mount" && fs.NArg() >= 2:
		mount(fs.Arg(0), fs.Arg(1))
	default:
		log.Println("unknown command:", cmd, fs.Args())
		usage()
	}
}
