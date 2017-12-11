package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type UrlResponse struct {
	WsUrl      string `json:"ws_url"`
	ProxyWsUrl string `json:"proxy_ws_url"`
	Error      string `json:"error"`
}

func GetVolumeWebsocketURL(volumePath, token string) (string, error) {
	client := &http.Client{Timeout: time.Duration(10) * time.Second}

	req, err := http.NewRequest("POST", hubURL()+"/volumes/"+volumePath, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "CFSToken "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result UrlResponse
	json.Unmarshal(b, &result)
	if result.Error != "" || result.WsUrl == "" {
		return "", fmt.Errorf("Error: %s", result.Error)
	}

	return result.WsUrl, nil
}

func GetProxyWsURL(volumePath, token string) (string, error) {
	client := &http.Client{Timeout: time.Duration(10) * time.Second}

	req, err := http.NewRequest("GET", hubURL()+"/volumes/"+volumePath, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "CFSToken "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result UrlResponse
	json.Unmarshal(b, &result)
	if result.Error != "" || result.WsUrl == "" {
		return "", fmt.Errorf("Error: %s", result.Error)
	}

	return result.ProxyWsUrl, nil
}

func ConnectVolume(volumePath, token string) (*websocket.Conn, error) {
	wsurl, err := GetVolumeWebsocketURL(volumePath, token)
	if err != nil {
		return nil, err
	}

	user := strings.SplitN(volumePath, "/", 2)[0]
	return Connect(wsurl, user, token)
}

func ConnectViaPloxy(volumePath, token string) (*websocket.Conn, error) {
	wsurl, err := GetProxyWsURL(volumePath, token)
	if err != nil {
		return nil, err
	}

	return Connect(wsurl, "", token)
}

func Connect(wsurl, user, token string) (*websocket.Conn, error) {
	u, err := url.Parse(wsurl)
	if err != nil {
		return nil, err
	}

	rawConn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}

	wsHeaders := http.Header{
		//"Origin":                   {"http://localhost:8080"},
		"Sec-WebSocket-Extensions": {"permessage-deflate; client_max_window_bits, x-webkit-deflate-frame"},
	}

	wsConn, resp, err := websocket.NewClient(rawConn, u, wsHeaders, 1024, 1024)
	if err != nil {
		fmt.Errorf("websocket.NewClient Error: %s\nResp:%+v", err, resp)
		return nil, err
	}
	if user == "" {
		return wsConn, err
	}

	wsConn.WriteJSON(&map[string]string{"action": "auth", "user": user, "token": token})
	var authResult map[string]string
	err = wsConn.ReadJSON(&authResult)
	if authResult["status"] != "ok" {
		return nil, fmt.Errorf("auth error: %s", authResult["status"])
	}

	return wsConn, err
}
