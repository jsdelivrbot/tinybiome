package tinybiome

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

// types: 0 vitamin, 1 mineral
type Pellet struct {
	X    int64
	Y    int64
	Room *Room
	ID   int
	TID  int
	Mass int64
	Type int64
}

func (p *Pellet) Create() {
	p.Mass = 3
	for _, b := range p.Room.Connections {
		if b == nil {
			continue
		}
		b.Protocol.WriteNewPellet(p)
	}
	p.ID = p.Room.PelletCount
	p.Room.Pellets[p.ID] = p
	p.Room.PelletCount += 1
	tx := int(p.X / int64(TileSize))
	ty := int(p.Y / int64(TileSize))

	t := p.Room.PelletTiles[tx][ty]
	p.TID = t.PelletCount
	if len(t.Pellets) <= p.TID {
		t.Pellets = append(t.Pellets, p)
	} else {
		t.Pellets[p.TID] = p
	}
	t.PelletCount += 1
}
func (p *Pellet) Remove() {
	for _, b := range p.Room.Connections {
		if b == nil {
			continue
		}
		b.Protocol.WriteDestroyPellet(p)
	}
	r := p.Room
	r.PelletCount -= 1
	r.Pellets[p.ID] = r.Pellets[r.PelletCount]
	r.Pellets[p.ID].ID = p.ID

	tx := int(p.X / int64(TileSize))
	ty := int(p.Y / int64(TileSize))

	t := p.Room.PelletTiles[tx][ty]
	t.PelletCount -= 1
	t.Pellets[p.TID] = t.Pellets[t.PelletCount]
	t.Pellets[p.TID].TID = p.TID

}

type Actor struct {
	Owner     interface{}
	Room      *Room
	ID        int64
	X         float64
	Y         float64
	Direction float64
	Speed     float64
	Mass      float64
	Dead      bool

	XSpeed float64
	YSpeed float64
	radius float64
	oldm   float64
	oldx   float64
	oldy   float64
}

func NewActor(r *Room) *Actor {
	a := &Actor{Room: r}
	id := r.getId(a)
	a.ID = id
	return a
}
func (a *Actor) RecalcRadius() {
	a.radius = math.Pow(a.Mass, a.Room.Config.SizeMultiplier)
}
func (a *Actor) Radius() float64 {
	return a.radius
}

func (a *Actor) CheckCollisions() {
	if a.Dead {
		return
	}
	if pc, is := a.Owner.(PelletCollider); is {
		r := a.Radius()
		rad := a.Radius() * a.Radius()
		t := time.Now()

		ix := ((a.X - r) / TileSize)
		iy := ((a.Y - r) / TileSize)
		ax := (a.X)
		ay := (a.Y)
		ex := ((a.X + r) / TileSize)
		ey := ((a.Y + r) / TileSize)

		if ix < 0 {
			ix = 0
		}
		if iy < 0 {
			iy = 0
		}
		if ex > a.Room.Config.Width/TileSize-1 {
			ex = a.Room.Config.Width/TileSize - 1
		}
		if ey > a.Room.Config.Height/TileSize-1 {
			ey = a.Room.Config.Height/TileSize - 1
		}

		for ix := int(ix); ix <= int(ex); ix += 1 {
			for iy := int(iy); iy <= int(ey); iy += 1 {

				tile := a.Room.PelletTiles[ix][iy]
				pels := tile.Pellets[:tile.PelletCount]

				for _, p := range pels {
					dx := float64(p.X) - ax
					dy := float64(p.Y) - ay
					if dx*dx+dy*dy < rad {
						pc.PelletCollision(p)
					}
				}
			}
		}
		took := time.Since(t)
		if took > 1*time.Millisecond {
			log.Println("QR TOOK", took)
		}
	}

	if ac, is := a.Owner.(ActorCollider); is {
		consumes := []*Actor{}
		for _, b := range a.Room.Actors[:a.Room.HighestID] {
			if b == a || b == nil {
				continue
			}
			if b.Dead {
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
				if ac.ShouldCollide(b) {
					tot := a.Mass + b.Mass

					a.X -= dx / dist * depth * b.Mass / tot
					a.Y -= dy / dist * depth * b.Mass / tot
					b.X += dx / dist * depth * a.Mass / tot
					b.Y += dy / dist * depth * a.Mass / tot

					a.XSpeed = ((a.XSpeed - math.Min(dx/dist*depth/2*b.Mass/tot*35, 6)) + a.XSpeed) / 2
					a.YSpeed = ((a.YSpeed - math.Min(dy/dist*depth/2*b.Mass/tot*35, 6)) + a.YSpeed) / 2
					b.XSpeed = ((b.XSpeed + math.Min(dx/dist*depth/2*a.Mass/tot*35, 6)) + b.XSpeed) / 2
					b.YSpeed = ((b.YSpeed + math.Min(dy/dist*depth/2*a.Mass/tot*35, 6)) + b.YSpeed) / 2

					// b.Direction = math.Atan2(b.YSpeed, b.XSpeed)
					// b.Speed = math.Sqrt(b.XSpeed*b.XSpeed + b.YSpeed*b.YSpeed)
					// a.Direction = math.Atan2(a.YSpeed, a.XSpeed)
					// a.Speed = math.Sqrt(a.XSpeed*a.XSpeed + a.YSpeed*a.YSpeed)
					continue
				}
				if dist < a.Radius() || dist < b.Radius() {
					consumes = append(consumes, b)
				}
			}
		}

		if len(consumes) > 0 {
			for i := 0; i < len(consumes); i += 1 {
				if !a.Dead {
					ac.ActorCollision(consumes[i])
				}
			}
		}
	}
	a.X = math.Min(float64(a.Room.Config.Width), a.X)
	a.Y = math.Min(float64(a.Room.Config.Height), a.Y)
	a.X = math.Max(0, a.X)
	a.Y = math.Max(0, a.Y)
}

var friction = .2

func (a *Actor) Tick(d time.Duration) {
	allowed := 500 / (a.Room.Config.SpeedMultiplier * math.Pow(a.Mass+50, .5))
	distance := allowed * d.Seconds() * a.Speed

	dx := math.Cos(a.Direction) * distance
	dy := math.Sin(a.Direction) * distance

	a.XSpeed = math.Pow(friction, d.Seconds()) * a.XSpeed
	a.YSpeed = math.Pow(friction, d.Seconds()) * a.YSpeed
	a.X += a.XSpeed * d.Seconds()
	a.Y += a.YSpeed * d.Seconds()

	a.X += dx
	a.Y += dy
}

func (a *Actor) Write(p ProtocolDown) {
	p.WriteNewActor(a)
	p.WriteSetMassActor(a)
}

func (a *Actor) String() string {
	return fmt.Sprintf("%d (m:%f)", a.ID, a.Mass)
}

func (a *Actor) Remove() {
	r := a.Room
	log.Println("REMOVING ACTOR", a)
	r.Actors[a.ID] = nil
	a.Dead = true
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		conn.Protocol.WriteDestroyActor(a)
	}
}

type PelletCollider interface {
	PelletCollision(*Pellet)
}

type ActorCollider interface {
	ActorCollision(*Actor)
	ShouldCollide(*Actor) bool
}

type PlayerActor struct {
	Actor      *Actor
	Player     *Player
	MergeTime  time.Time
	DecayLevel float64
}

func (a *PlayerActor) Tick(d time.Duration) {
	a.Decay(d)
	a.Actor.Tick(d)
}

func (a *PlayerActor) Remove() {
	for n, oActor := range a.Player.Owns {
		if oActor == a {
			a.Player.Owns[n] = nil
			break
		}
	}
	a.Actor.Remove()
}
func (oa *PlayerActor) Spit() {
	if oa.Actor.Mass > 50 {
		oa.Actor.Mass -= 15
		oa.Actor.RecalcRadius()
		NewBlob(oa)
	}
}
func (oa *PlayerActor) Split() {
	a := oa.Actor
	if a.Mass < a.Room.Config.MinSplitMass {
		return
	}
	emptySlots := 0
	for _, n := range oa.Player.Owns {
		if n == nil {
			emptySlots += 1
		}
	}
	if emptySlots < 1 {
		return
	}

	a.Mass *= .5
	oa.MergeTime = oa.MergeTime.Add(oa.Player.Room.MergeTimeFromMass(a.Mass))
	a.RecalcRadius()

	distance := a.Radius()
	dx := math.Cos(a.Direction)
	dy := math.Sin(a.Direction)

	b := oa.Player.NewPlayerActor(a.X+dx*distance*2, a.Y+dy*distance*2, a.Mass)

	b.Actor.Direction = a.Direction
	b.Actor.Speed = a.Speed

	d := math.Sqrt(distance*40) * 7
	b.Actor.XSpeed = dx * d
	b.Actor.YSpeed = dy * d
}

func (a *PlayerActor) String() string {
	return fmt.Sprintf("PA %s (%s)", a.Player, a.Actor)
}

func (a *PlayerActor) ActorCollision(b *Actor) {
	if otherPlayerActor, isPlayer := b.Owner.(*PlayerActor); isPlayer {
		if !a.CanEat(otherPlayerActor) {
			return
		}
		log.Println(a, "CONSUMES", b)
		otherActor := otherPlayerActor.Actor
		if a.Player == otherPlayerActor.Player {
			a.Actor.Mass += otherActor.Mass
			a.MergeTime = a.MergeTime.Add(a.Player.Room.MergeTimeFromMass(otherActor.Mass))
		} else {
			a.Actor.Mass += otherActor.Mass * .65
			a.DecayLevel -= otherActor.Mass / a.Actor.Mass
		}
		a.Actor.RecalcRadius()
		otherPlayerActor.Remove()
	}
}

func (a *PlayerActor) PelletCollision(b *Pellet) {
	if b.Type == 0 {
		a.DecayLevel -= 10 * float64(b.Mass) / a.Actor.Mass
	} else {
		a.Actor.Mass += float64(b.Mass) * .9
	}
	a.Actor.RecalcRadius()
	b.Remove()
}

func (a *PlayerActor) ShouldCollide(b *Actor) bool {
	if otherPlayerActor, isPlayer := b.Owner.(*PlayerActor); isPlayer {
		if a.Player == otherPlayerActor.Player {
			if a.CanMerge() && otherPlayerActor.CanMerge() {
				return false
			}
			return true
		} else {
			return a.Player.ClanName == otherPlayerActor.Player.ClanName && a.Player.ClanName != ""
		}
	}
	return false
}

func (a *PlayerActor) Decay(d time.Duration) {
	if a.Actor.Mass > 150 {
		a.DecayLevel += (a.Actor.Mass - 150) / 1000000
	}
	if a.DecayLevel < -1 {
		a.DecayLevel = -1
	}
	if a.DecayLevel > 1 {
		a.DecayLevel = 1
	}
	if a.DecayLevel > (a.Actor.Mass-150)/300 {
		a.DecayLevel = (a.Actor.Mass - 150) / 300
	}

	if a.DecayLevel > 0 {
		a.Actor.Mass -= a.DecayLevel
	}
}

func (pa *PlayerActor) CanEat(pb *PlayerActor) bool {
	a := pa.Actor
	b := pb.Actor
	if pa.Player == pb.Player {
		if pa.CanMerge() && pb.CanMerge() {
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

func (a *PlayerActor) CanMerge() bool {
	return a.MergeTime.Before(time.Now())
}

func (pa *PlayerActor) Write(p ProtocolDown) {
	pa.Actor.Write(p)
	p.WritePlayerActor(pa)
}

type Blob struct {
	Actor  *Actor
	Origin *PlayerActor
	Birth  time.Time
	Room   *Room
}

func NewBlob(p *PlayerActor) *Blob {
	log.Println("SHOOT", p)
	r := p.Player.Room
	v := &Blob{Birth: time.Now(), Origin: p}
	v.Room = r

	v.Actor = r.NewActor(
		p.Actor.X,
		p.Actor.Y, 15)
	v.Actor.Owner = v
	v.Actor.Direction = p.Actor.Direction

	rad := p.Actor.Radius()
	v.Actor.XSpeed = math.Cos(p.Actor.Direction) * 300
	v.Actor.YSpeed = math.Sin(p.Actor.Direction) * 300
	v.Actor.X += math.Cos(p.Actor.Direction) * rad
	v.Actor.Y += math.Sin(p.Actor.Direction) * rad

	r.AddTicker(v)
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		v.Actor.Write(conn.Protocol)
		conn.Protocol.WriteBlob(v)
	}
	return v
}
func (b *Blob) ActorCollision(a *Actor) {
	b.Remove()
	a.Mass += b.Actor.Mass
	a.RecalcRadius()
	if av, is := a.Owner.(*Virus); is {
		av.LastDirection = b.Actor.Direction
		log.Println("FEEDING VIRUS, DIRECTION", b.Actor.Direction)
	}
}
func (b *Blob) ShouldCollide(a *Actor) bool {
	if pa, is := a.Owner.(*PlayerActor); is {
		if pa == b.Origin && time.Since(b.Birth) < time.Second*5 {
			return true
		}
		return false
	}
	if _, is := a.Owner.(*Blob); is {
		return true
	}
	return false
}
func (b *Blob) Remove() {
	b.Actor.Remove()
	b.Room.RemoveTicker(b)
}
func (b *Blob) Write(p ProtocolDown) {
	b.Actor.Write(p)
	p.WriteBlob(b)
}
func (b *Blob) Tick(d time.Duration) {
	b.Actor.Tick(d)
}

type Bacteria struct {
	Actor   *Actor
	Room    *Room
	TargetX float64
	TargetY float64
}

func NewBacteria(r *Room) *Bacteria {
	v := &Bacteria{}
	v.Room = r
	v.Actor = r.NewActor(
		rand.Float64()*r.Config.Width,
		rand.Float64()*r.Config.Height,
		350)
	v.Actor.Owner = v
	v.Actor.Direction = rand.Float64() * math.Pi * 2
	v.PickSpot()
	r.AddTicker(v)
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		v.Actor.Write(conn.Protocol)
		conn.Protocol.WriteBacteria(v)
	}
	v.Room.BacteriaCount += 1
	return v
}
func (v *Bacteria) PickSpot() {
	v.TargetX = v.Actor.X + rand.Float64()*500 - 250
	v.TargetY = v.Actor.Y + rand.Float64()*500 - 250
}
func (v *Bacteria) Write(p ProtocolDown) {
	v.Actor.Write(p)
	p.WriteBacteria(v)
}
func (v *Bacteria) ActorCollision(a *Actor) {
	if asPA, isPA := a.Owner.(*PlayerActor); isPA {
		if v.Actor.Mass < asPA.Actor.Mass {
			v.Actor.Mass += 1
			asPA.Actor.Mass -= 1
		} else {
			v.Actor.Mass -= 1
			asPA.Actor.Mass += 1
		}
		v.Actor.RecalcRadius()
		asPA.Actor.RecalcRadius()

		if v.Actor.Mass < 25 {
			v.Remove()
		}
	}
	if virus, isV := a.Owner.(*Virus); isV {
		v.Actor.Mass -= 1
		virus.Actor.Mass += 1

		v.Actor.RecalcRadius()
		virus.Actor.RecalcRadius()

		if v.Actor.Mass < 25 {
			v.Remove()
		}

		virus.LastEat = time.Now()
	}
}
func (v *Bacteria) ShouldCollide(a *Actor) bool {
	if _, isBacteria := a.Owner.(*Bacteria); isBacteria {
		return true
	}
	return false
}
func (v *Bacteria) Tick(d time.Duration) {
	if rand.Intn(100) == 0 {
		v.PickSpot()
	}

	dx := v.TargetX - v.Actor.X
	dy := v.TargetY - v.Actor.Y
	v.Actor.Direction = math.Atan2(dy, dx)

	dist := math.Sqrt(dx*dx + dy*dy)

	if dist > 1 {
		v.Actor.Speed += rand.Float64()*dist/1000*(1-v.Actor.Speed) - .03
		if v.Actor.Speed < 0 {
			v.Actor.Speed = 0
		}
		if v.Actor.Speed > .4 {
			v.Actor.Speed = .4
		}
	} else {
		v.Actor.Speed *= .5
	}

	v.Actor.Tick(d)
}
func (v *Bacteria) Remove() {
	v.Actor.Remove()
	v.Room.BacteriaCount -= 1
	v.Room.RemoveTicker(v)
}
func (v *Bacteria) String() string {
	return fmt.Sprintf("BA (%s)", v.Actor)
}

type Virus struct {
	Actor         *Actor
	Room          *Room
	TargetX       float64
	TargetY       float64
	LastEat       time.Time
	LastDirection float64
}

func NewVirus(r *Room) *Virus {
	v := &Virus{LastEat: time.Now()}
	v.Room = r
	v.Actor = r.NewActor(
		rand.Float64()*r.Config.Width,
		rand.Float64()*r.Config.Height,
		250)
	v.Actor.Owner = v
	v.Actor.Direction = rand.Float64() * math.Pi * 2
	v.PickSpot()
	r.AddTicker(v)
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		v.Actor.Write(conn.Protocol)
		conn.Protocol.WriteVirus(v)
	}
	v.Room.VirusCount += 1
	return v
}

func NewVirusWithSpecs(r *Room, x, y, mass float64) *Virus {
	v := &Virus{LastEat: time.Now()}
	v.Room = r
	v.Actor = r.NewActor(
		x, y, mass)
	v.Actor.Owner = v
	v.Actor.Direction = rand.Float64() * math.Pi * 2
	v.PickSpot()
	r.AddTicker(v)
	for _, conn := range r.Connections {
		if conn == nil {
			continue
		}
		v.Actor.Write(conn.Protocol)
		conn.Protocol.WriteVirus(v)
	}
	v.Room.VirusCount += 1
	return v
}

func (v *Virus) PickSpot() {
	v.TargetX = v.Actor.X + rand.Float64()*500 - 250
	v.TargetY = v.Actor.Y + rand.Float64()*500 - 250
}
func (v *Virus) Write(p ProtocolDown) {
	v.Actor.Write(p)
	p.WriteVirus(v)
}
func (v *Virus) ActorCollision(a *Actor) {
	if a.Mass > v.Actor.Mass {
		if asPA, isPA := a.Owner.(*PlayerActor); isPA {
			for range [6]byte{} {
				asPA.Split()
			}
			v.Remove()
			asPA.Actor.Mass += v.Actor.Mass * .1
			asPA.Actor.RecalcRadius()
		}
	}
}
func (v *Virus) ShouldCollide(a *Actor) bool {
	if ov, isVirus := a.Owner.(*Virus); isVirus {
		if time.Since(v.LastEat) > time.Second && time.Since(ov.LastEat) > time.Second {
			return true
		}
	}
	return false
}
func (v *Virus) Tick(d time.Duration) {
	if v.Actor.Mass > 340 {
		v.Actor.Mass -= 150
		nv := NewVirusWithSpecs(v.Room, v.Actor.X, v.Actor.Y, 150)
		nv.Actor.Direction = v.LastDirection
		log.Println("FEEDING VIRUS, DIRECTION", v.LastDirection)
		nv.Actor.XSpeed = math.Cos(nv.Actor.Direction) * 450
		nv.Actor.YSpeed = math.Sin(nv.Actor.Direction) * 450
		v.Actor.RecalcRadius()
	}
	if time.Since(v.LastEat) > time.Minute {
		v.Actor.Mass -= 1
		v.Actor.RecalcRadius()
	}

	if rand.Intn(100) == 0 {
		v.PickSpot()
	}

	dx := v.TargetX - v.Actor.X
	dy := v.TargetY - v.Actor.Y
	v.Actor.Direction = math.Atan2(dy, dx)

	dist := math.Sqrt(dx*dx + dy*dy)

	if dist > 1 {
		v.Actor.Speed += rand.Float64()*dist/1000*(1-v.Actor.Speed) - .03
		if v.Actor.Speed < 0 {
			v.Actor.Speed = 0
		}
		if v.Actor.Speed > .4 {
			v.Actor.Speed = .4
		}
	} else {
		v.Actor.Speed *= .5
	}

	v.Actor.Tick(d)
	if v.Actor.Mass < 10 {
		v.Remove()
	}
}
func (v *Virus) Remove() {
	v.Actor.Remove()
	v.Room.VirusCount -= 1
	v.Room.RemoveTicker(v)
}
func (v *Virus) String() string {
	return fmt.Sprintf("VI (%s)", v.Actor)
}
