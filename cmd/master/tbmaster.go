package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
)

var servers = make(map[*server]struct{})
var clients = make(map[*client]struct{})
var slock sync.RWMutex

var allowedCidrs = []string{
	"10.0.0.0/8",
	"192.0.0.0/8",
}

func checkHost(ip string) bool {
	n := net.ParseIP(ip)
	if n.IsLoopback() {
		log.Println(ip, "IS LOOPBACK")
		return true
	}
	for _, p := range allowedCidrs {
		if _, cidr, _ := net.ParseCIDR(p); cidr.Contains(n) {
			log.Println(ip, "IN ALLOWED CIDR")
			return true
		}
	}
	c := exec.Command("ssh-keygen", "-H", "-F", ip)
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	if e := c.Start(); e != nil {
		log.Panicln("FAILED TO FIND SSH-KEYGEN", e)
	}
	if e := c.Wait(); e != nil {
		log.Println("HACK ATTEMPT?", e, ip)
		return false
	}
	log.Println(n, "ALLOWED BECAUSE IN SSH ALLOWED HOSTS")
	return true
}

func main() {

	m := http.NewServeMux()
	m.Handle("/", websocket.Handler(newConn))
	go http.ListenAndServe("0.0.0.0:4000", m)

	d, _ := os.Getwd()
	log.Println(os.StartProcess(d+"/tbnode", nil, &os.ProcAttr{}))

	{
		w := http.NewServeMux()
		fs := http.FileServer(http.Dir("./ui"))
		w.Handle("/", fs)

		log.Println("ABOUT TO LISTEN FOR HTTP")
		go http.ListenAndServe("0.0.0.0:8080", w)
	}
	{
		w := http.NewServeMux()
		fs := http.FileServer(http.Dir("./ui"))
		w.Handle("/", fs)

		log.Println("ABOUT TO LISTEN FOR HTTP")
		http.ListenAndServe("0.0.0.0:80", w)
	}
}

type server struct {
	ip   string
	port int
}

func (s *server) addr() string {
	return fmt.Sprintf("%s:%d", s.ip, s.port)
}

type client struct {
	ws *websocket.Conn
	w  *json.Encoder
}

func newConn(ws *websocket.Conn) {
	ra := ws.Request().RemoteAddr
	ip, _, _ := net.SplitHostPort(ra)
	if ip == "::1" {
		ip = "127.0.0.1"
	}
	if ip == "127.0.0.1" {
		ip = GetLocalIP()
		log.Println("LOCAL IP DETECTED AS", ip)
	}
	j := json.NewDecoder(ws)
	w := json.NewEncoder(ws)

	cli := &client{ws, w}
	slock.RLock()
	clients[cli] = struct{}{}
	log.Println("NEW CLIENT", ip)
	for s, _ := range servers {
		w.Encode(map[string]interface{}{"meth": "add", "address": s.addr()})
	}
	slock.RUnlock()

	var p *server
	var v map[string]interface{}
	for {
		e := j.Decode(&v)
		if e != nil {
			log.Println("ERR", e)
			break
		}
		switch v["meth"].(string) {
		case "addme":
			if checkHost(ip) {
				p = &server{ip: ip, port: int(v["port"].(float64))}
				log.Println("NEW SERVER", p)
				slock.Lock()
				servers[p] = struct{}{}
				for c, _ := range clients {
					c.w.Encode(map[string]interface{}{"meth": "add", "address": p.addr()})
				}
				slock.Unlock()
			} else {
				ws.Close()
				break
			}
		case "ping":

		}
	}

	slock.Lock()

	delete(clients, cli)
	if p != nil {
		delete(servers, p)
		log.Println("SERVER LEAVING", p, "# SERVERS", len(servers))
		for c, _ := range clients {
			c.w.Encode(map[string]interface{}{"meth": "del", "address": p.addr()})
		}
	}

	slock.Unlock()
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
