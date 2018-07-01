package tinybiome

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"time"
)

type Player struct {
	ID       int64
	Room     *Room
	Owns     []Ticker
	Name     string
	ClanName string
}

func NewPlayer(r *Room, name string) *Player {
	r.PlayerCount += 1
	player := &Player{Room: r, Owns: make([]Ticker, r.Config.MaxSplit)}
	player.ID = r.getPlayerId(player)
	player.Room.AddTicker(player)
	player.Name = name

	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		player.Write(conn.Protocol)
	}

	return player
}
func (p *Player) Tick(d time.Duration) {
	for _, actor := range p.Owns {
		if actor != nil {
			actor.Tick(d)
		}
	}
}

func (p *Player) Write(pn ProtocolDown) {
	pn.WriteNewPlayer(p)
	for _, actor := range p.Owns {
		if actor != nil {
			actor.Write(pn)
		}
	}
}

func (p *Player) UpdateDirection(actor int32, d, s float32) {
	if p.Room == nil {
		panic("player has no room")
	}
	r := p.Room
	p.Room.ChangeLock.RLock()
	defer p.Room.ChangeLock.RUnlock()

	a := r.getActor(int64(actor))
	if a == nil {
		fmt.Sprintf("ACTOR %#v not found?", actor)
		return
	}

	op, isPA := a.Owner.(*PlayerActor)

	if !isPA {
		log.Println(a, "APPARENTLY NOT A PA")
		return
	}

	if op.Player != p {
		log.Println(a, "APPARENTLY NOT OWNED BY", p)
		return
	}

	a.Direction = float64(d)
	a.Speed = float64(s)
	if a.Speed > 1 {
		a.Speed = 1
	}
	if a.Speed < 0 {
		a.Speed = 0
	}
}

type PlayerActorList []Ticker

func (a PlayerActorList) Len() int      { return len(a) }
func (a PlayerActorList) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PlayerActorList) Less(i, j int) bool {
	if a[i] == nil {
		return true
	}
	if a[j] == nil {
		return true
	}
	aA := a[i].(*PlayerActor)
	aB := a[j].(*PlayerActor)
	return aA.Actor.Mass > aB.Actor.Mass
}

func (p *Player) Split() {
	p.Room.ChangeLock.Lock()
	defer p.Room.ChangeLock.Unlock()
	n := make([]Ticker, len(p.Owns[:]))
	copy(n, p.Owns[:])
	sorted := PlayerActorList(n)
	sort.Sort(sorted)
	for _, a := range sorted {
		if a != nil {
			a.(*PlayerActor).Split()
		}
	}
}
func (p *Player) Spit() {
	p.Room.ChangeLock.Lock()
	defer p.Room.ChangeLock.Unlock()
	for _, a := range p.Owns {
		if a != nil {
			a.(*PlayerActor).Spit()
		}
	}
}

func (p *Player) Remove() {
	r := p.Room
	log.Println("Lock 4")
	for _, ticker := range p.Owns {
		if ticker == nil {
			continue
		}
		ticker.Remove()
	}
	r.Players[p.ID] = nil
	r.RemoveTicker(p)
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		conn.Protocol.WriteDestroyPlayer(p)
	}
	r.PlayerCount -= 1
	log.Println("Unlock 4")
	log.Println("CLOSING CHAN")
}

func (p *Player) NewPlayerActor(x, y, mass float64) *PlayerActor {
	r := p.Room
	playerActor := &PlayerActor{
		Actor:     r.NewActor(x, y, mass),
		Player:    p,
		MergeTime: time.Now().Add(r.MergeTimeFromMass(mass)),
	}
	playerActor.Actor.Owner = playerActor

	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		playerActor.Write(conn.Protocol)
	}

	for n, a := range p.Owns {
		if a == nil {
			p.Owns[n] = playerActor
			break
		}
	}

	return playerActor
}

func (p *Player) String() string {
	return fmt.Sprintf("PL #%d (%s)", p.ID, p.Name)
}

var clanRegex = regexp.MustCompile(`^\[(.*)\]`)

func (p *Player) Rename(n string) {
	if len(n) > 100 {
		n = n[:100]
	}
	p.Name = n

	names := clanRegex.FindStringSubmatch(n)
	if names != nil {
		p.ClanName = names[0]
		log.Println(p, "JOINED CLAN", p.ClanName)
	}

	for _, conn := range p.Room.Connections {
		if conn == nil {
			continue
		}
		conn.Protocol.WriteNamePlayer(p)
	}
}
