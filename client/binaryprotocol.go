package client

import (
	"bufio"
	"errors"
	"io"
	"log"
	"math/rand"
	"sync"
	"time"
	"unsafe"
)

type BinaryProtocol struct {
	RW    io.ReadWriter
	W     *bufio.Writer
	R     *bufio.Reader
	Flush bool
	T     time.Time
	Count int
	Lock  sync.RWMutex
}

func NewBinaryProtocol(ws io.ReadWriter) Protocol {
	return Protocol(&BinaryProtocol{RW: ws,
		W:     bufio.NewWriterSize(ws, 1024*10),
		R:     bufio.NewReaderSize(ws, 1024*1024),
		T:     time.Now(),
		Flush: true})
}

var invalidProto = errors.New("Invalid Protocol")

// runs in a goroutine
func (s *BinaryProtocol) GetMessage(p *Player) error {
	r := p.room
	act, e := s.R.ReadByte()
	if e != nil {
		return e
	}
	switch act {
	case 0:
		size := make([]byte, 4)
		s.R.Read(size)
		strSize := *(*int32)(unsafe.Pointer(&size[0]))
		log.Println(p, "SENT LENGTH", strSize)
		if strSize > 100 {
			strSize = 100
		}
		if strSize < 0 {
			return invalidProto
		}
		nameBytes := make([]byte, strSize)
		s.R.Read(nameBytes)
		name := string(nameBytes)
		for _, n := range p.Owns {
			if n != nil {
				return nil
			}
		}
		p.Rename(name)
		log.Println(name, "JOINED")
		p.NewActor(rand.Float64()*float64(r.Width), rand.Float64()*float64(r.Height), float64(r.StartMass))
	case 1:
		parts := make([]byte, 12)
		s.R.Read(parts)
		pid := int64(*(*int32)(unsafe.Pointer(&parts[0])))
		d := *(*float32)(unsafe.Pointer(&parts[4]))
		s := *(*float32)(unsafe.Pointer(&parts[8]))
		p := r.getActor(pid)
		if p != nil {
			p.Direction = float64(d)
			p.Speed = float64(s)
			if p.Speed > 1 {
				p.Speed = 1
			}
			if p.Speed < 0 {
				p.Speed = 0
			}
		}
	case 2:
		for _, a := range p.Owns {
			if a != nil {
				a.Split()
			}
		}
	}
	return nil
	// r := p.room
	// decoded := map[string]interface{}{}
	// err := s.R.Decode(&decoded)
	// if time.Since(s.T) > 100*time.Millisecond {
	// 	s.T = time.Now()
	// 	s.Count = 0
	// }
	// s.Count += 1

	// if err != nil {
	// 	return err
	// }
	// if s.Count > 100 {
	// 	log.Println("DROPPING")
	// 	return nil
	// }

	// switch decoded["type"] {
	// case "move":
	// 	pid := int(decoded["id"].(float64))
	// 	p := r.getActor(pid)
	// 	if p != nil {
	// 		p.Direction = decoded["d"].(float64)
	// 		p.Speed = decoded["s"].(float64)
	// 		if p.Speed > 1 {
	// 			p.Speed = 1
	// 		}
	// 		if p.Speed < 0 {
	// 			p.Speed = 0
	// 		}
	// 	}
	// case "split":
	// 	for _, a := range p.Owns {
	// 		if a != nil {
	// 			a.Split()
	// 		}
	// 	}
	// case "join":
	// 	p.Rename(decoded["name"].(string))
	// 	for _, n := range p.Owns {
	// 		if n != nil {
	// 			return nil
	// 		}
	// 	}
	// 	p.NewActor(rand.Float64()*float64(r.Width), rand.Float64()*float64(r.Height), float64(r.StartMass))
	// }
	return nil
}

func (s *BinaryProtocol) done() {
	if s.Flush {
		s.W.Flush()
	} else {
		s.check()
	}
}

func (s *BinaryProtocol) check() {
	if s.W.Available() < 100 {
		s.W.Flush()
	}
}

func WriteBytes(w io.Writer, p unsafe.Pointer, l int) {
	switch l {
	case 1:
		a := (*[1]byte)(p)[:]
		w.Write(a)
	case 2:
		a := (*[2]byte)(p)[:]
		w.Write(a)
	case 4:
		a := (*[4]byte)(p)[:]
		w.Write(a)
	}
}

func WriteInt32(w io.Writer, i int64) {
	j := int32(i)
	w.Write((*[4]byte)(unsafe.Pointer(&j))[:])
}
func WriteFloat32(w io.Writer, i float64) {
	j := float32(i)
	w.Write((*[4]byte)(unsafe.Pointer(&j))[:])
}

// sends updates
func (s *BinaryProtocol) WriteRoom(r *Room) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(0)
	WriteInt32(s.W, r.Width)
	WriteInt32(s.W, r.Height)
	WriteInt32(s.W, r.StartMass)
	WriteInt32(s.W, r.MergeTime)
	s.done()
}

func (s *BinaryProtocol) WriteNewActor(actor *Actor) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(1)
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.X)
	WriteFloat32(s.W, actor.Y)
	WriteFloat32(s.W, actor.Mass)
	WriteInt32(s.W, actor.Player.ID)
	s.done()
	// newPlayer := `{"type":"new","x":%f,"y":%f,"id":%d,"mass":%f,"owner":%d}`
	// dat := fmt.Sprintf(newPlayer, actor.X, actor.Y, actor.ID, actor.Mass, actor.Player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteNewPellet(pellet *Pellet) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(2)
	WriteInt32(s.W, pellet.X)
	WriteInt32(s.W, pellet.Y)
	WriteInt32(s.W, pellet.Type)
	s.done()
}

func (s *BinaryProtocol) WriteDestroyPellet(pellet *Pellet) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(3)
	WriteInt32(s.W, pellet.X)
	WriteInt32(s.W, pellet.Y)
	s.done()
	// str := `{"type":"delpel","x":%d,"y":%d}`
	// dat := fmt.Sprintf(str, pellet.X, pellet.Y)
	// _ = dat
}

func (s *BinaryProtocol) WriteNewPlayer(player *Player) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(4)
	WriteInt32(s.W, player.ID)
	b := []byte(player.Name)
	l := len(b)
	WriteInt32(s.W, int64(l))
	s.W.Write(b)
	s.done()
	// o := map[string]interface{}{"type": "addplayer",
	// 	"id": player.ID, "name": player.Name}
	// s.W.Encode(o)
}

func (s *BinaryProtocol) WriteNamePlayer(player *Player) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(5)
	WriteInt32(s.W, player.ID)
	b := []byte(player.Name)
	l := len(b)
	WriteInt32(s.W, int64(l))
	s.W.Write(b)
	s.done()
	// o := map[string]interface{}{"type": "nameplayer",
	// 	"id": player.ID, "name": player.Name}
	// s.W.Encode(o)
}

func (s *BinaryProtocol) WriteDestroyPlayer(player *Player) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(6)
	WriteInt32(s.W, player.ID)
	s.done()
	// str := `{"type":"delplayer","id":%d}`
	// dat := fmt.Sprintf(str, player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteOwns(player *Player) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(7)
	WriteInt32(s.W, player.ID)
	s.done()
	// ownsPlayer := `{"type":"own","id":%d}`
	// dat := fmt.Sprintf(ownsPlayer, player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteDestroyActor(actor *Actor) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(8)
	WriteInt32(s.W, actor.ID)
	s.done()
	// delPlayer := `{"type":"del","id":%d}`
	// dat := fmt.Sprintf(delPlayer, actor.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteMoveActor(actor *Actor) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(9)
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.X)
	WriteFloat32(s.W, actor.Y)
	WriteFloat32(s.W, actor.Direction)
	WriteFloat32(s.W, actor.Speed)
	s.done()
	// delPlayer := `{"type":"move","id":%d,"x":%f,"y":%f,"d":%f,"s":%f}`
	// dat := fmt.Sprintf(delPlayer, actor.ID, actor.X, actor.Y, actor.Direction, actor.Speed)
	// _ = dat
}

func (s *BinaryProtocol) WriteSetMassActor(actor *Actor) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}
	s.W.WriteByte(10)
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.Mass)
	s.done()
	// delPlayer := `{"type":"mass","id":%d,"mass":%f}`
	// dat := fmt.Sprintf(delPlayer, actor.ID, actor.Mass)
	// _ = dat
}

func (s *BinaryProtocol) WritePelletsIncoming(pellets []*Pellet) {
	if s.Flush {
		s.Lock.Lock()
		defer s.Lock.Unlock()
	}

	s.W.WriteByte(11)
	WriteInt32(s.W, int64(len(pellets)))
	for _, pel := range pellets {
		WriteInt32(s.W, pel.X)
		WriteInt32(s.W, pel.Y)
		WriteInt32(s.W, pel.Type)
	}

	s.done()
}

func (s *BinaryProtocol) MultiStart() {
	s.W = bufio.NewWriterSize(s.RW, 1024*1024*10)
	s.Lock.Lock()
	s.Flush = false
}

func (s *BinaryProtocol) MultiSteal(p Protocol) {
}

func (s *BinaryProtocol) MultiSend() {
	s.W.Flush()
	s.Flush = true
	s.W = bufio.NewWriterSize(s.RW, 1024*10)
	s.Lock.Unlock()
}
