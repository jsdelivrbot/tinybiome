package client

import (
	"golang.org/x/net/websocket"
	"log"
	"net"
	"net/http"
	"sync"
)

type Server struct {
	Room *Room
	Lock sync.RWMutex
	IPS  map[string]int
	WSH  websocket.Handler
}

func NewServer() *Server {
	cli := &Server{IPS: map[string]int{}}
	cli.WSH = websocket.Handler(cli.Accept)
	return cli
}

var allowedHosts = map[string]struct{}{
	"http://www.tinybio.me":   struct{}{},
	"http://localhost:8080":   struct{}{},
	"http://96.50.20.37:8080": struct{}{},
}

func (s *Server) Handler(res http.ResponseWriter, req *http.Request) {
	a := req.RemoteAddr
	ip, _, _ := net.SplitHostPort(a)

	s.Lock.Lock()
	if n, found := s.IPS[ip]; found {
		if n >= 8 {
			res.WriteHeader(400)
			s.Lock.Unlock()
			log.Println("REJECTING", ip)
			return
		}
		s.IPS[ip] = n + 1
	} else {
		s.IPS[ip] = 0
	}

	s.Lock.Unlock()

	log.Println("New Client", ip)
	s.WSH.ServeHTTP(res, req)

	log.Println("CLIENT LEAVING", ip)
	s.Lock.Lock()
	s.IPS[ip] -= 1
	s.Lock.Unlock()
}

func (s *Server) Accept(ws *websocket.Conn) {
	if _, found := allowedHosts[ws.RemoteAddr().String()]; !found {
		log.Println("REJECTED BECAUSE", ws.RemoteAddr().String())
		return
	}
	ws.PayloadType = websocket.BinaryFrame
	room := s.Room
	room.Accept(NewBinaryProtocol(ws))

}

func (s *Server) AddRoom(r *Room) {
	s.Room = r
}
