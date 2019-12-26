package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/binzume/cfs/fuse"
	"github.com/binzume/cfs/volume"
	"github.com/binzume/cfs/ws_volume"
)

var defaultHubAPI = "http://localhost:8080"
var defaultHubToken = "dummysecret"

func hubURL() string {
	url := os.Getenv("CFS_HUB_URL")
	if url == "" {
		url = defaultHubAPI
	}
	return url
}

func hubToken() string {
	token := os.Getenv("CFS_HUB_TOKEN")
	if token == "" {
		token = defaultHubToken
	}
	return token
}

func usage() {
	log.Printf("usage: cs help [command]")
	log.Printf("       cs publish local user/volume")
	log.Printf("       cs mount user/volume mountpoint")
}

func publish(localPath, volumePath string, writable bool) error {
	log.Println("publish ", localPath, " to ", volumePath)

	hubConn, err := ConnectVolume(volumePath, hubToken())
	if err != nil {
		log.Println(err)
		return err
	}
	defer hubConn.Close()
	finish := make(chan error)
	hubConn.WriteJSON(&map[string]string{"action": "volume", "name": strings.SplitN(volumePath, "/", 2)[1], "url": "ws://localhost:8080/"})

	v := volume.NewLocalVolume(localPath) // volumePath, writable

	go func() {
		// listen loop
		defer close(finish)
		for {
			var event map[string]string
			err := hubConn.ReadJSON(&event)
			if err != nil {
				log.Println("read error: ", err)
				finish <- err
			}
			log.Println(event)
			if event["action"] == "connect" {
				newConn, err := Connect(event["ws_url"], "", "") // TODO
				if err == nil {
					go ws_volume.ConnectClient(v, newConn, event["target"])
				}
			}
		}
	}()

	log.Println("wait...")
	<-finish
	log.Println("finished.")
	return nil
}

func mount(volumePath, mountPoint string) error {
	log.Println("mount ", volumePath, " to ", mountPoint)

	connector := func(v *ws_volume.RemoteVolume) (*websocket.Conn, error) {
		return ConnectViaPloxy(v.Name, hubToken())
	}

	v := ws_volume.NewRemoteVolume(volumePath, connector)
	volumeExit, err := v.Start()
	if err != nil {
		log.Println("connect error: ", err)
		return err
	}
	defer v.Terminate()

	mountErr := fuse.MountVolume(v, mountPoint)

	log.Println("started.", err)
	select {
	case err = <-volumeExit:
		log.Println("disconnected: ", err)
	case err = <-mountErr:
		log.Println("unmoount: ", err)
	}
	log.Println("finished.")
	return nil
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
