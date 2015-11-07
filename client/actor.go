package client

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// types: 0 vitamind, 1 mineral
type Pellet struct {
	X    int64
	Y    int64
	room *Room
	ID   int64
	Mass int64
	Type int64
}

func (p *Pellet) Create() {
	p.Mass = 3
	done := p.room.Write("Creating Pellet")
	if p.room.PelletCount < MaxPellets {
		for _, b := range p.room.Players {
			if b == nil {
				continue
			}
			b.Net.WriteNewPellet(p)
		}
		p.ID = p.room.PelletCount
		p.room.Pellets[p.ID] = p
		p.room.PelletCount += 1
	}
	done()
}
func (p *Pellet) Remove() {
	done := p.room.Write("Removing Pellet")
	for _, b := range p.room.Players {
		if b == nil {
			continue
		}
		b.Net.WriteDestroyPellet(p)
	}
	r := p.room
	r.PelletCount -= 1
	r.Pellets[p.ID] = r.Pellets[r.PelletCount]
	r.Pellets[p.ID].ID = p.ID

	done()

}

type Actor struct {
	ID        int64
	X         float64
	Y         float64
	Direction float64
	Speed     float64
	moved     bool
	Mass      float64

	XSpeed float64
	YSpeed float64

	Player     *Player
	MergeTime  time.Time
	radius     float64
	LastUpdate time.Time
	DecayLevel float64
}

func (a *Actor) Decay() {
	m := a.Mass
	if a.DecayLevel < a.Mass {
		a.DecayLevel += (a.Mass - 200) / 10000
	}
	if a.DecayLevel > a.Mass {
		a.DecayLevel = a.Mass
	}
	if a.DecayLevel < 0 {
		a.DecayLevel = 0
	}
	a.Mass -= a.DecayLevel / 15
	if a.Mass < 20 {
		a.Mass = 20
	}
	if a.Mass != m {
		a.Player.Net.WriteSetMassActor(a)
	}
}
func (a *Actor) RecalcRadius() {
	a.radius = math.Pow(a.Mass/math.Pi, a.Player.room.SizeMultiplier)
}
func (a *Actor) Radius() float64 {
	return a.radius
}

func (a *Actor) Move(x, y float64) {
	a.X = x
	a.Y = y

	done := a.Player.room.Read("Eating pellets")
	pellets := []*Pellet{}
	r := a.Player.room
	for _, p := range r.Pellets[:r.PelletCount] {
		dx := float64(p.X) - a.X
		dy := float64(p.Y) - a.Y
		dist := dx*dx + dy*dy
		allowedDist := 3 + a.Radius()
		if dist < allowedDist*allowedDist {
			pellets = append(pellets, p)
		}
	}

	done()

	if len(pellets) > 0 {
		for i := 0; i < len(pellets); i += 1 {
			a.ConsumePellet(pellets[i])
		}
	}

	consumes := []*Actor{}
	done = a.Player.room.Read("Moving player")
	for _, b := range a.Player.room.Actors[:a.Player.room.HighestID] {
		if b == a || b == nil {
			continue
		}
		dx := b.X - a.X
		dy := b.Y - a.Y
		dist := dx*dx + dy*dy
		if dist == 0 {
			dist = .01
		}
		allowedDist := a.Radius() + b.Radius()
		if dist < allowedDist*allowedDist {
			dist = math.Sqrt(dist)
			depth := allowedDist - dist
			if a.MustCollide(b) {
				tot := a.Mass + b.Mass
				a.XSpeed = ((a.XSpeed - dx/dist*depth/2*b.Mass/tot) + a.XSpeed) / 2
				a.YSpeed = ((a.YSpeed - dy/dist*depth/2*b.Mass/tot) + a.YSpeed) / 2
				b.XSpeed = ((b.XSpeed + dx/dist*depth/2*a.Mass/tot) + b.XSpeed) / 2
				b.YSpeed = ((b.YSpeed + dy/dist*depth/2*a.Mass/tot) + b.YSpeed) / 2
				a.Player.Net.WriteMoveActor(a)
			}
			if (dist < a.Radius() || dist < b.Radius()) && a.CanEat(b) {
				log.Println(a, "EATS", b)
				consumes = append(consumes, b)
			}
		}
	}
	done()

	if len(consumes) > 0 {
		eating := a.Player.room.Write("Eating other actors")
		for i := 0; i < len(consumes); i += 1 {
			a.Consume(consumes[i])
		}
		eating()
	}
	a.moved = true
	a.X = math.Min(float64(a.Player.room.Width), a.X)
	a.Y = math.Min(float64(a.Player.room.Height), a.Y)
	a.X = math.Max(0, a.X)
	a.Y = math.Max(0, a.Y)
}

var friction = .1

func (a *Actor) Tick(d time.Duration) {
	allowed := 100 / (math.Pow(.5*a.Mass, .1))
	distance := allowed * d.Seconds() * a.Speed

	dx := math.Cos(a.Direction) * distance
	dy := math.Sin(a.Direction) * distance

	a.XSpeed = math.Pow(friction, d.Seconds()) * a.XSpeed
	a.YSpeed = math.Pow(friction, d.Seconds()) * a.YSpeed
	a.X += a.XSpeed
	a.Y += a.YSpeed

	a.Move(a.X+dx, a.Y+dy)

}

func (a *Actor) CanMerge() bool {
	return a.MergeTime.Before(time.Now())
}

func (a *Actor) MustCollide(b *Actor) bool {
	if a.Player == b.Player {
		if a.CanMerge() && b.CanMerge() {
			return false
		}
		return true
	}
	return false
}

func (a *Actor) CanEat(b *Actor) bool {
	if a.Player == b.Player {
		if a.CanMerge() && b.CanMerge() {
			log.Println(a, "CAN MERGE", b)
			if a.Mass >= b.Mass {
				return true
			} else {
				log.Println(a, "CANT MERGE BECAUSE MASS", b)
			}
		}
		return false
	}
	if float64(a.Mass) > float64(b.Mass)*1.10 {
		return true
	}
	return false
}

func (a *Actor) Consume(b *Actor) {
	log.Println(a, "CONSUMES", b)
	if a.Player == b.Player {
		a.Mass += b.Mass
		a.MergeTime = a.MergeTime.Add(a.Player.room.MergeTimeFromMass(b.Mass))
	} else {
		a.Mass += b.Mass * .65
		a.DecayLevel = 0
	}
	a.RecalcRadius()
	b.Player.EditLock.Lock()
	b.Remove()
	b.Player.EditLock.Unlock()
	a.Player.Net.WriteSetMassActor(a)
}

func (a *Actor) ConsumePellet(b *Pellet) {
	if b.Type == 0 {
		a.DecayLevel /= 2
	} else {
		a.Mass += float64(b.Mass) * .9
	}
	a.RecalcRadius()
	b.Remove()
	a.Player.Net.WriteSetMassActor(a)
}

func (a *Actor) String() string {
	return fmt.Sprintf("%d (m:%f, o:%s)", a.ID, a.Mass, a.Player)
}

func (a *Actor) Split() {
	if a.Mass < 40 {
		return
	}
	emptySlots := 0
	for _, n := range a.Player.Owns {
		if n == nil {
			emptySlots += 1
		}
	}
	if emptySlots < 1 {
		return
	}

	a.Player.EditLock.Lock()
	nb := a.Player.NewActor(a.X, a.Y, a.Mass*.5)
	nb.Direction = a.Direction
	nb.Speed = a.Speed

	distance := math.Sqrt(nb.Radius()*2) * 1
	XSpeed := math.Cos(a.Direction)
	YSpeed := math.Sin(a.Direction)

	a.Remove()
	b := a.Player.NewActor(a.X+XSpeed*nb.Radius(), a.Y+YSpeed*nb.Radius(), a.Mass*.5)
	a.Player.EditLock.Unlock()

	b.Direction = a.Direction
	b.Speed = a.Speed

	b.XSpeed = XSpeed * distance
	b.YSpeed = YSpeed * distance
}

func (a *Actor) Remove() {
	for n, oActor := range a.Player.Owns {
		if oActor == a {
			a.Player.Owns[n] = nil
			break
		}
	}

	r := a.Player.room
	r.Actors[a.ID] = nil
	for _, player := range r.Players {
		if player == nil {
			continue
		}
		player.Net.WriteDestroyActor(a)
	}
}

type Player struct {
	ID       int64
	Net      Protocol
	room     *Room
	Owns     [MaxOwns]*Actor
	EditLock sync.RWMutex
	Name     string
}

func (p *Player) NewActor(x, y, mass float64) *Actor {
	r := p.room
	actor := &Actor{X: x, Y: y, Player: p, Mass: mass,
		MergeTime: time.Now().Add(r.MergeTimeFromMass(mass))}
	actor.RecalcRadius()
	id := r.getId(actor)
	actor.ID = id
	log.Println("NEW ACTOR", actor)

	done := r.Read("Updating players on new actor")
	for _, oPlayer := range r.Players {
		if oPlayer == nil {
			continue
		}
		oPlayer.Net.WriteNewActor(actor)
	}
	done()

	for n, a := range p.Owns {
		if a == nil {
			p.Owns[n] = actor
			break
		}
	}

	return actor
}

func (p *Player) Remove() {
	r := p.room
	p.EditLock.Lock()
	done := r.Write("Removing player and players actors")
	for _, actor := range r.Actors {
		if actor == nil {
			continue
		}
		if actor.Player == p {
			actor.Remove()
		}
	}
	p.EditLock.Unlock()
	r.Players[p.ID] = nil
	for _, oPlayer := range r.Players {
		if oPlayer == nil {
			continue
		}
		oPlayer.Net.WriteDestroyPlayer(p)
	}
	done()
}

func (p *Player) String() string {
	return fmt.Sprintf("#%d (%s)", p.ID, p.Name)
}

func (p *Player) Rename(n string) {
	if len(n) > 100 {
		n = n[:100]
	}
	p.Name = n
	done := p.room.Read("Updating name to player")
	for _, oPlayer := range p.room.Players {
		if oPlayer == nil {
			continue
		}
		oPlayer.Net.WriteNamePlayer(p)
	}
	done()
}
