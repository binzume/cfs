package main

import (
	"flag"
	"log"
	"os"
	"time"

	"./volume"
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

func publish(localPath, volumePath string, writable bool) error {
	log.Println("publish ", localPath, " to ", volumePath)

	hubConn, err := ConnectVolume(volumePath, hubToken())
	if err != nil {
		log.Println(err)
		return err
	}
	defer hubConn.Close()
	finish := make(chan int)
	hubConn.WriteJSON(&map[string]string{"action": "volume", "name": "fuga", "url": "ws://localhost:8080/"})

	v := volume.NewLocalVolume(localPath, volumePath, writable)

	go func() {
		// listen loop
		for {
			var event map[string]string
			err := hubConn.ReadJSON(&event)
			if err != nil {
				log.Println("read error: ", err)
				break
			}
			log.Println(event)
			if event["action"] == "connect" {
				newConn, _ := Connect(event["ws_url"], "", "") // TODO
				go volume.ConnectClient(v, newConn, event["target"])
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

	v := volume.NewRemoteVolume(volumePath, conn)
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
