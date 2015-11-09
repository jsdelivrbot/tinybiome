package client

import (
	"fmt"
	"github.com/mleveck/go-quad-tree"
	"log"
	"math"
	"time"
)

// types: 0 vitamind, 1 mineral
type Pellet struct {
	X      int64
	Y      int64
	room   *Room
	ID     int64
	Mass   int64
	Type   int64
	QPoint *qtree.Point
}

func (p *Pellet) Create() {
	p.QPoint = qtree.NewPoint(float64(p.X), float64(p.Y), p)
	p.Mass = 3
	for _, b := range p.room.Players {
		if b == nil {
			continue
		}
		b.Net.WriteNewPellet(p)
	}
	p.ID = p.room.PelletCount
	p.room.Pellets[p.ID] = p
	p.room.PelletCount += 1
	p.room.PelletQuadTree.Insert(p.QPoint)
}
func (p *Pellet) Remove() {
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
	p.room.PelletQuadTree.Remove(p.QPoint)
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

func (a *Actor) Decay(d time.Duration) {
	if a.Mass > 150 {
		a.DecayLevel += (a.Mass - 150) / 1000000
	}
	if a.DecayLevel < -1 {
		a.DecayLevel = -1
	}
	if a.DecayLevel > 1 {
		a.DecayLevel = 1
	}
	if a.DecayLevel > (a.Mass-150)/300 {
		a.DecayLevel = (a.Mass - 150) / 300
	}

	if a.DecayLevel > 0 {
		a.Mass -= a.DecayLevel
	}
}
func (a *Actor) RecalcRadius() {
	a.radius = math.Pow(a.Mass/math.Pi, a.Player.room.SizeMultiplier)
}
func (a *Actor) Radius() float64 {
	return a.radius
}

func (a *Actor) CheckCollisions() {
	r := a.Player.room
	rad := int64(a.Radius() * a.Radius())
	nb := qtree.NewBounds(a.X-a.Radius(), a.Y-a.Radius(), a.Radius()*2, a.Radius()*2)
	pels := r.PelletQuadTree.QueryRange(nb)
	ix := int64(a.X)
	iy := int64(a.Y)
	for _, n := range pels {
		p := n.Val.(*Pellet)
		dx := p.X - ix
		dy := p.Y - iy
		if dx*dx+dy*dy < rad {
			a.ConsumePellet(p)
		}
	}

	consumes := []*Actor{}
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

				a.X -= dx / dist * depth * b.Mass / tot
				a.Y -= dy / dist * depth * b.Mass / tot
				b.X += dx / dist * depth * a.Mass / tot
				b.Y += dy / dist * depth * a.Mass / tot

				a.XSpeed = ((a.XSpeed - math.Min(dx/dist*depth/2*b.Mass/tot, 6)) + a.XSpeed) / 2
				a.YSpeed = ((a.YSpeed - math.Min(dy/dist*depth/2*b.Mass/tot, 6)) + a.YSpeed) / 2
				b.XSpeed = ((b.XSpeed + math.Min(dx/dist*depth/2*a.Mass/tot, 6)) + b.XSpeed) / 2
				b.YSpeed = ((b.YSpeed + math.Min(dy/dist*depth/2*a.Mass/tot, 6)) + b.YSpeed) / 2
			}
			if (dist < a.Radius() || dist < b.Radius()) && a.CanEat(b) {
				log.Println(a, "EATS", b)
				consumes = append(consumes, b)
			}
		}
	}

	if len(consumes) > 0 {
		for i := 0; i < len(consumes); i += 1 {
			a.Consume(consumes[i])
		}
	}
	a.moved = true
	a.X = math.Min(float64(a.Player.room.Width), a.X)
	a.Y = math.Min(float64(a.Player.room.Height), a.Y)
	a.X = math.Max(0, a.X)
	a.Y = math.Max(0, a.Y)
}

var friction = .1

func (a *Actor) Tick(d time.Duration) {
	allowed := 500 / (a.Player.room.SpeedMultiplier * math.Pow(a.Mass+50, .5))
	distance := allowed * d.Seconds() * a.Speed

	dx := math.Cos(a.Direction) * distance
	dy := math.Sin(a.Direction) * distance

	a.XSpeed = math.Pow(friction, d.Seconds()) * a.XSpeed
	a.YSpeed = math.Pow(friction, d.Seconds()) * a.YSpeed
	a.X += a.XSpeed
	a.Y += a.YSpeed

	a.X += dx
	a.Y += dy

}

func (a *Actor) CanMerge() bool {
	return a.MergeTime.Before(time.Now())
}

func (a *Actor) MustCollide(b *Actor) bool {
	if a.Player.ClanName == b.Player.ClanName && a.Player.ClanName != "" {
		return true
	}
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

func (a *Actor) MoveTo(dx, dy float64) {
	this.direction = Math.atan2(mdy,mdx)
	this.speed = Math.sqrt(mdx*mdx+mdy*mdy)
	if (this.speed<.2) this.speed=0
	if (this.speed>1) this.speed=1
}

func (a *Actor) Consume(b *Actor) {
	log.Println(a, "CONSUMES", b)
	if a.Player == b.Player {
		a.Mass += b.Mass
		a.MergeTime = a.MergeTime.Add(a.Player.room.MergeTimeFromMass(b.Mass))
	} else {
		a.Mass += b.Mass * .65
		a.DecayLevel -= b.Mass / a.Mass
	}
	a.RecalcRadius()
	b.Remove()
}

func (a *Actor) ConsumePellet(b *Pellet) {
	if b.Type == 0 {
		a.DecayLevel -= 10 * float64(b.Mass) / a.Mass
	} else {
		a.Mass += float64(b.Mass) * .9
	}
	a.RecalcRadius()
	b.Remove()
}

func (a *Actor) String() string {
	return fmt.Sprintf("%d (m:%f, o:%s)", a.ID, a.Mass, a.Player)
}

func (a *Actor) OldSplit() {
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

	nb := a.Player.NewActor(a.X, a.Y, a.Mass*.5)
	nb.Direction = a.Direction
	nb.Speed = a.Speed

	distance := math.Sqrt(nb.Radius()*2) * 1
	XSpeed := math.Cos(a.Direction)
	YSpeed := math.Sin(a.Direction)

	a.Remove()
	b := a.Player.NewActor(a.X+XSpeed*nb.Radius(), a.Y+YSpeed*nb.Radius(), a.Mass*.5)

	b.Direction = a.Direction
	b.Speed = a.Speed

	d := math.Sqrt(distance)
	b.XSpeed = XSpeed * d
	b.YSpeed = YSpeed * d
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

	a.Mass *= .5
	a.MergeTime = a.MergeTime.Add(a.Player.room.MergeTimeFromMass(a.Mass))
	a.RecalcRadius()

	distance := math.Sqrt(a.Radius()*3) * 1
	XSpeed := math.Cos(a.Direction)
	YSpeed := math.Sin(a.Direction)

	b := a.Player.NewActor(a.X+XSpeed*a.Radius(), a.Y+YSpeed*a.Radius(), a.Mass)

	b.Direction = a.Direction
	b.Speed = a.Speed

	d := math.Sqrt(distance)
	b.XSpeed = XSpeed * d
	b.YSpeed = YSpeed * d
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
