package tinybiome

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
)

const TickLen = 25
const TileSize = 100

type Ticker interface {
	Tick(time.Duration)
	Write(ProtocolDown)
	Remove()
}

type Tile struct {
	PelletCount int
	Pellets     []*Pellet
}

func NewTile() *Tile {
	return &Tile{Pellets: make([]*Pellet, 0)}
}

type RoomConfig struct {
	Name            string  `json:"name"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
	MaxBacteria     int     `json:"maxbacteria"`
	MaxViruses      int     `json:"maxviruses"`
	MaxSplit        int     `json:"maxsplit"`
	MaxPellets      int     `json:"maxpellets"`
	MergeTime       float64 `json:"mergetime"`
	SizeMultiplier  float64 `json:"sizemultiplier"`
	SpeedMultiplier float64 `json:"speedmultiplier"`
	StartingMass    float64 `json:"startmass"`
	MinSplitMass    float64 `json:"minsplitmass"`
}

func (c *RoomConfig) String() string {
	return fmt.Sprintf("%fx%f, %dV, %dB", c.Width, c.Height, c.MaxViruses, c.MaxBacteria)
}

type Room struct {
	ID          int
	Config      *RoomConfig
	Actors      []*Actor
	Connections []*Connection
	Players     []*Player
	Tickers     []Ticker
	HighestID   int64
	Pellets     []*Pellet
	PelletTiles [][]*Tile
	PelletCount int
	PlayerCount int64

	VirusCount    int
	BacteriaCount int

	Ticker     *time.Ticker
	ChangeLock sync.RWMutex
}

func NewRoom(c *RoomConfig) *Room {
	r := &Room{
		Actors:      make([]*Actor, 256*256),
		Connections: make([]*Connection, 256*256),
		Players:     make([]*Player, 256*256),
		Tickers:     make([]Ticker, 256*256),
		Pellets:     make([]*Pellet, c.MaxPellets),
		Ticker:      time.NewTicker(time.Millisecond * TickLen),
		Config:      c,
	}
	r.CreateTiles()
	log.Println(r)

	return r
}

func (r *Room) run(d time.Duration) {
	t := time.Now()
	r.ChangeLock.Lock()
	r.createThings(d)
	r.checkCollisions()
	r.doTicks(d)
	r.sendUpdates()
	for _, conn := range r.Connections {
		if conn != nil {
			conn.Protocol.Save()
		}
	}
	r.ChangeLock.Unlock()
	took := time.Since(t)
	if took > time.Millisecond*10 {
		log.Println("TICK TOOK", took)
	}
}
func (r *Room) checkCollisions() {
	t := time.Now()
	for _, actor := range r.Actors[:r.HighestID] {
		if actor == nil {
			continue
		}
		actor.CheckCollisions()
	}
	took := time.Since(t)
	if took > time.Millisecond*1 {
		log.Println("CHECKCOLS TOOK", took, "FOR", r.HighestID)
	}
}
func (r *Room) createThings(d time.Duration) {
	perSecond := (1 - math.Pow(float64(r.PelletCount)/float64(r.Config.MaxPellets), 2)) * r.Config.Width * r.Config.Height / 100
	for i := int64(0); i < int64(d.Seconds()*perSecond); i++ {
		if r.PelletCount < r.Config.MaxPellets {
			newPel := &Pellet{
				X:    int64(rand.Intn(int(r.Config.Width))),
				Y:    int64(rand.Intn(int(r.Config.Height))),
				Type: int64(rand.Intn(2)),
				Room: r,
			}
			newPel.Create()
		}
	}

	if r.VirusCount < r.Config.MaxViruses {
		NewVirus(r)
	}
	if r.BacteriaCount < r.Config.MaxBacteria {
		NewBacteria(r)
	}
}

func (r *Room) doTicks(d time.Duration) {
	t := time.Now()
	for _, player := range r.Tickers {
		if player != nil {
			player.Tick(d)
		}
	}
	took := time.Since(t)
	if took > time.Millisecond*1 {
		log.Println("DOTICK TOOK", took)
	}
}

func (r *Room) sendUpdates() {
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		for _, actor := range r.Actors[:r.HighestID] {
			if actor == nil {
				continue
			}
			if actor.X != actor.oldx || actor.Y != actor.oldy {
				conn.Protocol.WriteMoveActor(actor)
			}
			if actor.oldm != actor.Mass {
				conn.Protocol.WriteSetMassActor(actor)
			}
		}
	}
	for _, actor := range r.Actors[:r.HighestID] {
		if actor == nil {
			continue
		}
		actor.oldx = actor.X
		actor.oldy = actor.Y
		actor.oldm = actor.Mass
	}
}

func (r *Room) String() string {
	return fmt.Sprintf(`R#%d`, r.ID)
}

func (r *Room) CreateTiles() {
	r.PelletTiles = make([][]*Tile, int(r.Config.Width/TileSize))
	for i := int64(0); i < int64(r.Config.Width/TileSize); i += 1 {
		r.PelletTiles[i] = make([]*Tile, int(r.Config.Height/TileSize))
		for j := int64(0); j < int64(r.Config.Height/TileSize); j += 1 {
			r.PelletTiles[i][j] = NewTile()
		}
	}

	go func() {
		lastTick := time.Now()
		for range r.Ticker.C {
			now := time.Now()
			r.run(now.Sub(lastTick))
			lastTick = now
		}
	}()
}

func (r *Room) getId(a *Actor) int64 {
	for id64, found := range r.Actors {
		id := int64(id64)
		if found == nil {
			if id+1 > r.HighestID {
				r.HighestID = id + 1
			}
			r.Actors[id] = a
			return id
		}
	}
	log.Println("OUT OF ACTOR IDS?")
	return -1
}

func (r *Room) getPlayerId(p *Player) int64 {
	for a, found := range r.Players {
		id := int64(a)
		if found == nil {
			r.Players[id] = p
			return id
		}
	}
	log.Println("OUT OF PLAYER IDS?")
	return -1
}

func (r *Room) getActor(id int64) *Actor {
	if id < 0 {
		return nil
	}
	if id > r.HighestID {
		return nil
	}
	return r.Actors[id]
}

func (r *Room) MergeTimeFromMass(mass float64) time.Duration {
	s := r.Config.MergeTime * (1 + mass/2000)
	d := time.Duration(s*1000) * time.Millisecond
	log.Println("MERGE TIME FOR", mass, "=", s, "SECONDS =", d)

	return d
}
func (r *Room) NewActor(x, y, mass float64) *Actor {
	actor := NewActor(r)
	actor.X = x
	actor.Y = y
	actor.Mass = mass
	actor.RecalcRadius()
	log.Println("NEW ACTOR", actor)
	return actor
}
func (r *Room) AddTicker(t Ticker) {
	for n, o := range r.Tickers {
		if o == nil {
			r.Tickers[n] = t
			return
		}
	}
}
func (r *Room) RemoveTicker(t Ticker) {
	for n, o := range r.Tickers {
		if o == t {
			r.Tickers[n] = nil
			return
		}
	}
}
