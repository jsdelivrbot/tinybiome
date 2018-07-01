package tinybiome

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

type Server struct {
	Host string
	Port int

	Lock       sync.RWMutex
	IPS        map[string]int
	WSH        websocket.Handler
	Config     NodeConfig
	MainConfig Config
	Origins    map[string]struct{}

	LiveRooms []*LiveRoom
}

func NewServer(sc NodeConfig, main Config, rt *RetryGroup) *Server {
	cli := &Server{
		Config:     sc,
		MainConfig: main,
		Origins:    make(map[string]struct{}),
	}

	cli.Setup()

	for n, roomConfig := range sc.Rooms {
		lr := NewLiveRoom(roomConfig)
		lr.ID = n
		cli.LiveRooms = append(cli.LiveRooms, lr)
		rt.Add("room:"+roomConfig.Name, lr.Start)
	}
	for _, origin := range sc.Origins {
		cli.Origins[origin] = struct{}{}
	}

	log.Println("NEW SERVER", sc)
	rt.Add(sc.Name+":http", cli.RunHTTP)
	rt.Add(sc.Name+":comms", cli.CommunicateWithMaster)

	return cli
}

func (s *Server) String() string {
	return fmt.Sprintf("SRV %s", s.Config.Name)
}

func (s *Server) CommunicateWithMaster() error {
	d, e := websocket.Dial("ws://"+s.Config.Master, "", "http://server.go")
	if e != nil {
		return e
	}
	writer := json.NewEncoder(d)

	writer.Encode(map[string]interface{}{
		"meth":     "addme",
		"port":     s.Port,
		"host":     s.Host,
		"insecure": s.Config.Insecure,
	})
	for {
		time.Sleep(time.Second)
		if e := writer.Encode(map[string]interface{}{"meth": "ping"}); e != nil {
			return e
		}
	}
}

func (s *Server) Setup() error {

	hostString, portString, err := net.SplitHostPort(s.Config.Address)
	if err != nil {
		return err
	}
	portInt, err := strconv.Atoi(portString)
	if err != nil {
		return err
	}

	s.Host = hostString
	s.Port = portInt

	s.IPS = map[string]int{}
	s.WSH = websocket.Handler(s.Accept)
	return nil
}

func (s *Server) RunHTTP() error {
	m := http.NewServeMux()
	log.Println("WEBSOCKETS STARTING")
	m.HandleFunc("/", s.Handler)

	if s.Config.Insecure {
		if err := http.ListenAndServe(s.Config.Address, m); err != nil {
			log.Println("node err", err)
			return err
		}
		return nil
	}
	if err := http.ListenAndServeTLS(s.Config.Address, s.MainConfig.CertFile, s.MainConfig.KeyFile, m); err != nil {
		log.Println("node err", err)
		return err
	}
	return nil
}

func (s *Server) allowed(req *http.Request) bool {
	if checkHost(req.RemoteAddr) {
		return true
	}
	origin, err := url.Parse(req.Header.Get("Origin"))
	if err != nil {
		log.Println("rejecting because origin unparseable:", err.Error())
		return false
	}
	o := fmt.Sprintf("%s://%s", origin.Scheme, origin.Hostname())
	if _, found := s.Origins[o]; !found {
		log.Println("REJECTED BECAUSE ORIGIN NOT ALLOWED:", o)
		return false
	}

	return true
}

func (s *Server) Handler(res http.ResponseWriter, req *http.Request) {
	if !s.allowed(req) {
		res.WriteHeader(400)
		return
	}

	a := req.RemoteAddr
	ip, _, _ := net.SplitHostPort(a)
	s.Lock.Lock()
	if n, found := s.IPS[ip]; found {
		if n >= 8 {
			res.WriteHeader(400)
			s.Lock.Unlock()
			log.Println("REJECTING DUE TO DDOS-LIKE BEHAVIOUR", ip)
			return
		}
		s.IPS[ip] = n + 1
	} else {
		s.IPS[ip] = 0
	}

	s.Lock.Unlock()

	log.Println("CLIENT ENTERS", ip)

	defer func() {
		log.Println("CLIENT EXITS", ip)
		s.Lock.Lock()
		s.IPS[ip] -= 1
		s.Lock.Unlock()
	}()

	s.WSH.ServeHTTP(res, req)

}

func (s *Server) Accept(ws *websocket.Conn) {
	ws.PayloadType = websocket.BinaryFrame
	if _, err := NewConnection(s, ws); err != nil {
		log.Println("error accepting wss:", err.Error())
		ws.SetDeadline(time.Now().Add(2 * time.Millisecond))
		ws.Close()
	}
}
