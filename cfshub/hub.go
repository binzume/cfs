package main

// Storage Hub Server
// TODO: STUN + UDP

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var DefaultHubToken = "dummysecret"

func hubToken() string {
	Token := os.Getenv("CFS_HUB_TOKEN")
	if Token == "" {
		Token = DefaultHubToken
	}
	return Token
}

// Active volume
type Volume struct {
	Owner      string
	Name       string
	ConnectURL string
	event      chan *volumeEvent
	cmd        chan string // chan *Cmd
}

type volumeEvent struct {
	action string
	client *Client
	value  string
}

var volumes = map[string]*Volume{}

func NewVolume(owner, name, wsurl string, conn *websocket.Conn) *Volume {
	v := &Volume{Owner: owner, Name: name, ConnectURL: wsurl, event: make(chan *volumeEvent)}
	log.Printf("NewVolume %s %s", owner, name)
	volumes[v.Path()] = v // TODO: lock
	go volumeLoop(v, conn)
	return v
}

func (v *Volume) Path() string {
	return v.Owner + "/" + v.Name
}

func (v *Volume) Dispose() {
	log.Println("dispose", v.Path())
	delete(volumes, v.Path()) // TODO: lock
	v.cmd <- "close"
}

func (v *Volume) NewProxyConnection(id string) {
	v.event <- &volumeEvent{"connect", &Client{}, id}
}

func volumeLoop(v *Volume, conn *websocket.Conn) {
	for {
		select {
		case _ = <-v.cmd:
			return
		case ev := <-v.event:
			fmt.Println(ev.action)
			if ev.action == "connect" {
				conn.WriteJSON(map[string]string{"action": "connect", "ws_url": ev.value, "target": "file"})
			}
		}
	}
}

type Client struct {
}

type Item struct {
	Name string
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ProxyConnection struct {
	id           string
	conn1, conn2 *websocket.Conn
	done         bool
	wait         chan int
}

var connections = map[string]*ProxyConnection{}

func CloseProxyConnection(id string) {
	if c, ok := connections[id]; ok {
		delete(connections, id)
		c.Close()
	}
}

func CreateProxyConnection(id string, conn1 *websocket.Conn) *ProxyConnection {
	c := &ProxyConnection{id, conn1, nil, false, make(chan int)}
	connections[id] = c
	return c
}

func (c *ProxyConnection) Close() {
	if !c.done {
		c.done = true
		c.conn1.Close()
		if c.conn2 != nil {
			c.conn2.Close()
		}
		c.wait <- 1
	}
}

func (c *ProxyConnection) Join(conn2 *websocket.Conn) {
	c.conn2 = conn2
	go func() {
		defer CloseProxyConnection(c.id)
		c.conn1.WriteJSON(&map[string]string{})
		for {
			t, m, err := c.conn2.ReadMessage()
			if err != nil {
				break
			}
			err = c.conn1.WriteMessage(t, m)
			if err != nil {
				break
			}
		}
	}()
	go func() {
		defer CloseProxyConnection(c.id)
		// c.conn2.WriteJSON(&map[string]string{})
		for {
			t, m, err := c.conn1.ReadMessage()
			if err != nil {
				break
			}
			err = c.conn2.WriteMessage(t, m)
			if err != nil {
				break
			}
		}
	}()
}

func proxyWsHandler(v *Volume, cid string, w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if cid == "new" {
		cid = strconv.FormatUint(rand.Uint64(), 36)
		c := CreateProxyConnection(cid, conn)

		wsurl := fmt.Sprintf("ws://%s/volumes/%s/proxy/%s", r.Host, v.Path(), cid)
		v.event <- &volumeEvent{"connect", &Client{}, wsurl}
		log.Println("request connect", wsurl)
		<-c.wait
	} else {
		if c, ok := connections[cid]; ok {
			c.Join(conn)
			<-c.wait
		}
	}
}

func volumeWsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v", err)
		return
	}
	user := "guest"

	for {
		// t, msg, err := conn.ReadMessage()
		// conn.WriteMessage(t, msg)
		var data map[string]string
		err = conn.ReadJSON(&data)
		if err != nil {
			log.Println("readjson", err)
			break
		}
		log.Println(data)
		if data["action"] == "auth" {
			if data["token"] == hubToken() {
				user = data["user"]
				conn.WriteJSON(&map[string]string{"action": "response", "status": "ok"})
			} else {
				conn.WriteJSON(&map[string]string{"action": "response", "status": "invalid_token"})
			}
		}
		if user == "guest" {
			conn.WriteJSON(&map[string]string{"action": "response", "status": "auth required"})
			continue
		}

		if data["action"] == "volume" {
			v := NewVolume(user, data["name"], data["url"], conn)
			defer v.Dispose()
		}
	}
	log.Println("disconnect")
}

type FileResponse struct {
	Uri     string `json:"uri"`
	Name    string `json:"name"`
	Updated int64  `json:"updated"`
}

func parseIntDefault(str string, defvalue int) int {
	v, err := strconv.ParseInt(str, 10, 32)
	if err != nil {
		return defvalue
	}
	return int(v)
}

func wsUrl(r *http.Request, path string) string {
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		return "ws://" + r.Host + path
	}
	return "wss://" + r.Host + path
}

func initHttpd() *gin.Engine {
	r := gin.Default()

	r.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"_status": 200, "message": "It works!"})
	})

	r.Static("/css", "./static/css")
	r.Static("/js", "./static/js")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})
	r.POST("/volumes/:u/:v", func(c *gin.Context) {
		// TODO auth
		vpath := c.Param("u") + "/" + c.Param("v")
		wsurl := wsUrl(c.Request, "/volumes/"+vpath+"/ws")
		c.JSON(200, gin.H{"ws_url": wsurl})
	})

	r.GET("/volumes/:u/:v", func(c *gin.Context) {
		vpath := c.Param("u") + "/" + c.Param("v")
		log.Println(volumes)
		if v, ok := volumes[vpath]; ok {
			// TODO: if v.DisableProxy ...
			proxyWsURL := wsUrl(c.Request, "/volumes/"+vpath+"/proxy/new")
			c.JSON(200, gin.H{"ws_url": v.ConnectURL, "proxy_ws_url": proxyWsURL})
		} else {
			c.JSON(404, gin.H{"error": "notfound " + vpath})
		}
	})
	r.GET("/volumes/:u/:v/ws", func(c *gin.Context) {
		volumeWsHandler(c.Writer, c.Request)
	})
	r.GET("/volumes/:u/:v/proxy/:cid", func(c *gin.Context) {
		vpath := c.Param("u") + "/" + c.Param("v")
		if v, ok := volumes[vpath]; ok {
			proxyWsHandler(v, c.Param("cid"), c.Writer, c.Request)
		} else {
			c.JSON(404, gin.H{"error": "volume notfound:" + vpath})
		}
	})

	return r
}

func main() {
	port := flag.Int("p", 8080, "http port")
	flag.Parse()
	gin.SetMode(gin.ReleaseMode)
	log.Printf("start server. port: %d", *port)
	initHttpd().Run(":" + fmt.Sprint(*port))
}
