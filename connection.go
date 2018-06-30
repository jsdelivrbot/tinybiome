package tinybiome

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"
)

type Connection struct {
	Server   *Server
	Protocol Protocol
	Player   *Player
	Room     *LiveRoom
}

func NewConnection(s *Server, rwc io.ReadWriteCloser) (*Connection, error) {
	c := &Connection{s, NewBinaryProtocol(rwc), nil, nil}
	go c.SendUpdates()
	log.Println("SYNCING ROOMS")
	for _, room := range s.LiveRooms {
		c.Protocol.WriteRoom(room)
	}
	e := c.ReadForever()
	c.Done()
	return c, e
}

func (c *Connection) String() string {
	return fmt.Sprintf("CON %s ON %s (%s, P %s)", c.Protocol, c.Server, c.Room, c.Player)
}

func (c *Connection) ReadForever() error {
	for {
		reason := c.Protocol.GetMessage(c)
		if reason != nil {
			return reason
		}
	}
}

func (c *Connection) SendUpdates() {
	for {
		e := c.Protocol.Flush()
		if e != nil {
			log.Println("ERROR SENDING", e)
			break
		}
	}
}

func (c *Connection) Sync() {
	r := c.Room

	r.ChangeLock.Lock()
	r.Connections = append(r.Connections, c)

	log.Println("SYNCING", c.Player)
	start := time.Now()

	log.Println("SYNCING OTHER PLAYERS")
	for _, oPlayer := range r.Players {
		if oPlayer == nil {
			continue
		}
		c.Protocol.WriteNewPlayer(oPlayer)
	}

	log.Println("SYNCING TICKERS")
	for _, oPlayer := range r.Tickers {
		if oPlayer == nil {
			continue
		}
		if oPlayer == c.Player {
			continue
		}
		oPlayer.Write(c.Protocol)
	}

	log.Println("SYNCING PELLETS")
	c.Protocol.WritePelletsIncoming(r.Pellets[:r.PelletCount])
	took := time.Since(start)

	log.Println(c.Player, "INITIAL SYNC COMPLETE IN", took)
	c.Protocol.Save()

	r.ChangeLock.Unlock()
	log.Println(c.Player, "SENDING")
}

func (c *Connection) Done() {
	r := c.Room
	if r != nil {
		r.ChangeLock.Lock()
		for n, nc := range r.Connections {
			if nc == c {
				r.Connections[n] = r.Connections[len(r.Connections)-1]
				r.Connections = r.Connections[0 : len(r.Connections)-1]
				break
			}
		}
		if c.Player != nil {
			c.Player.Remove()
		}
		r.ChangeLock.Unlock()
	}
}

func (c *Connection) Ping() {
	if c.Room != nil {
		c.Room.ChangeLock.RLock()
	}
	c.Protocol.WritePong()
	c.Protocol.Save()

	for _, room := range c.Server.LiveRooms {
		c.Protocol.WriteRoom(room)
	}
	c.Protocol.Save()

	if c.Room != nil {
		c.Room.ChangeLock.RUnlock()
	}
}

func (c *Connection) Spectate(room int) {
	if c.Room != nil {
		log.Println("PLAYER ALREADY CREATED")
		return
	}

	c.Room = c.Server.LiveRooms[room]
	log.Println(c, "JOINING", c.Room)
	c.Sync()
}

func (c *Connection) StopSpectating() {
	log.Println(c, "STOPPING SPECTATOR")
	r := c.Room
	if r != nil {
		r.ChangeLock.Lock()
		for n, nc := range r.Connections {
			if nc == c {
				log.Println(c, "REMOVED FROM CONNECTIONS POOL")
				r.Connections[n] = r.Connections[len(r.Connections)-1]
				r.Connections = r.Connections[0 : len(r.Connections)-1]
				break
			}
		}
		if c.Player != nil {
			c.Player.Remove()
			log.Println(c, "PLAYER REMOVED")
			c.Player = nil
		}
		r.ChangeLock.Unlock()
		c.Room = nil
	}
	c.Protocol.WriteStopSpectating()
	c.Protocol.Save()
}

func (c *Connection) CreatePlayer(name string) {
	if c.Player == nil {
		r := c.Room
		r.ChangeLock.Lock()
		c.Player = NewPlayer(r.Room, name)
		c.Protocol.WriteOwns(c.Player)
		pa := c.Player.NewPlayerActor(rand.Float64()*r.Config.Width, rand.Float64()*r.Config.Height, r.Config.StartMass)
		c.Protocol.Save()
		r.ChangeLock.Unlock()
		log.Println(c, "JOINED AS", pa)
	}
}
