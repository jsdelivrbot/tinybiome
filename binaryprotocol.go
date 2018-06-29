package tinybiome

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"
	"unsafe"
)

var fallenBehind = errors.New("Socket fell behind")
var invalidProto = errors.New("Invalid Protocol")
var hackAttempt = errors.New("Hack Attempt")
var Disconnected = errors.New("Disconnected")

type ProtocolDown interface {
	WriteNewActor(*Actor)
	WriteDestroyActor(*Actor)
	WriteNewPlayer(*Player)
	WriteDestroyPlayer(*Player)
	WriteNamePlayer(*Player)
	WriteOwns(*Player)
	WriteRoom(*Room)
	WriteMoveActor(*Actor)
	WriteSetMassActor(*Actor)
	WriteNewPellet(*Pellet)
	WriteDestroyPellet(*Pellet)
	WritePelletsIncoming([]*Pellet)
	WritePlayerActor(*PlayerActor)
	WriteVirus(*Virus)
	WriteBacteria(*Bacteria)
	WriteBlob(*Blob)
	WritePong()
	WriteStopSpectating()
	Flush() error
	Save()
	SaveOob()
}

type ProtocolUp interface {
	GetMessage(*Connection) error
}

type Protocol interface {
	ProtocolDown
	ProtocolUp
}

type BinaryProtocol struct {
	RW            io.ReadWriter
	W             *bytes.Buffer
	OW            *bytes.Buffer
	R             *bufio.Reader
	isTransaction bool
	T             time.Time
	Count         int
	Lock          sync.RWMutex
	Logging       int
	DownLogging   int
	WriteChan     chan []byte
	OobWriteChan  chan []byte
	Disconnected  bool
	MessageMap    []byte
	CloseChan     chan error
}

func NewBinaryProtocol(ws io.ReadWriter) Protocol {
	p := &BinaryProtocol{
		RW:            ws,
		W:             bytes.NewBuffer(nil),
		OW:            bytes.NewBuffer(nil),
		R:             bufio.NewReaderSize(ws, 1024*4),
		T:             time.Now(),
		isTransaction: false,
		WriteChan:     make(chan []byte, 1000),
		OobWriteChan:  make(chan []byte, 100),
		CloseChan:     make(chan error, 100),
		Logging:       2,
		DownLogging:   2,
	}
	a := Protocol(p)
	p.WriteNewMessageMap()
	return a
}

func (s *BinaryProtocol) String() string {
	return "BINARY PROTO"
}

func guaranteeRead(r io.Reader, p []byte) error {
	_, e := io.ReadFull(r, p)
	return e
}

// runs in a goroutine
func (s *BinaryProtocol) GetMessage(c *Connection) error {
	if s.Disconnected {
		return Disconnected
	}
	act, e := s.R.ReadByte()
	if e != nil {
		log.Println(s, "ERR", e)
		// if e == io.EOF {
		// 	return nil
		// }
		return e
	}
	switch act {
	case 0:
		size := make([]byte, 4)
		e := guaranteeRead(s.R, size)
		if e != nil {
			return e
		}

		strSize := *(*int32)(unsafe.Pointer(&size[0]))
		if s.DownLogging > 0 {
			log.Println(c, "SENT JOIN")
		}
		if strSize > 100 {
			strSize = 100
		}
		if strSize < 0 {
			return invalidProto
		}
		nameBytes := make([]byte, strSize)
		e = guaranteeRead(s.R, nameBytes)
		if e != nil {
			return e
		}

		name := string(nameBytes)
		c.CreatePlayer(name)
	case 1: // movement
		parts := make([]byte, 12)
		e := guaranteeRead(s.R, parts)
		if e != nil {
			return e
		}

		pid := *(*int32)(unsafe.Pointer(&parts[0]))
		d := *(*float32)(unsafe.Pointer(&parts[4]))
		if math.IsNaN(float64(d)) {
			d = 0
		}

		speed := *(*float32)(unsafe.Pointer(&parts[8]))
		if math.IsNaN(float64(speed)) {
			speed = 0
		}
		if s.DownLogging > 1 {
			log.Println(c, "SENT DIRECTION", pid, d, speed)
		}
		if c.Player != nil {
			c.Player.UpdateDirection(pid, d, speed)
		}
	case 2:
		if s.DownLogging > 0 {
			log.Println(c, "SENT SPLIT")
		}
		if c.Player != nil {
			c.Player.Split()
		}
	case 3: // stop spectating
		if s.DownLogging > 0 {
			log.Println(c, "SENT STOP")
		}
		c.StopSpectating()
	case 4:
		if s.DownLogging > 0 {
			log.Println(c, "SENT PING")
		}
		c.Ping()
	case 5:
		if s.DownLogging > 0 {
			log.Println(c, "SENT SPIT")
		}
		if c.Player != nil {
			c.Player.Spit()
		}
	case 6: // spectate
		if s.DownLogging > 0 {
			log.Println(c, "SENT SPECTATE")
		}
		if room, e := s.R.ReadByte(); e != nil {
			return e
		} else {
			c.Spectate(int(room))
		}
	default:
		log.Println("READ ERROR, TYPE", act)
		return invalidProto
	}
	return nil
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
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteRoom", r)
	}
	s.W.WriteByte(s.MessageMap[0])
	WriteInt32(s.W, int64(r.ID))
	WriteFloat32(s.W, r.Config.Width)
	WriteFloat32(s.W, r.Config.Height)
	WriteFloat32(s.W, r.Config.StartingMass)
	WriteInt32(s.W, int64(r.Config.MergeTime))
	WriteFloat32(s.W, r.Config.SizeMultiplier)
	WriteFloat32(s.W, r.Config.SpeedMultiplier)
	WriteInt32(s.W, r.PlayerCount)

	nb := []byte(r.Config.Name)
	WriteInt32(s.W, int64(len(nb)))
	s.W.Write(nb)
}

func (s *BinaryProtocol) WriteNewActor(actor *Actor) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteNewActor", actor)
	}
	s.W.WriteByte(s.MessageMap[1])
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.X)
	WriteFloat32(s.W, actor.Y)
	WriteFloat32(s.W, actor.Mass)
	// newPlayer := `{"type":"new","x":%f,"y":%f,"id":%d,"mass":%f,"owner":%d}`
	// dat := fmt.Sprintf(newPlayer, actor.X, actor.Y, actor.ID, actor.Mass, actor.Player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteNewPellet(pellet *Pellet) {
	if s.Logging > 2 {
		log.Println(s, "SENDING WriteNewPellet", pellet)
	}
	s.W.WriteByte(s.MessageMap[2])
	WriteInt32(s.W, pellet.X)
	WriteInt32(s.W, pellet.Y)
	WriteInt32(s.W, pellet.Type)
}

func (s *BinaryProtocol) WriteDestroyPellet(pellet *Pellet) {
	if s.Logging > 2 {
		log.Println(s, "SENDING WriteDestroyPellet", pellet)
	}
	s.W.WriteByte(s.MessageMap[3])
	WriteInt32(s.W, pellet.X)
	WriteInt32(s.W, pellet.Y)
	// str := `{"type":"delpel","x":%d,"y":%d}`
	// dat := fmt.Sprintf(str, pellet.X, pellet.Y)
	// _ = dat
}

func (s *BinaryProtocol) WriteNewPlayer(player *Player) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteNewPlayer", player)
	}
	s.W.WriteByte(s.MessageMap[4])
	WriteInt32(s.W, player.ID)
	b := []byte(player.Name)
	l := len(b)
	WriteInt32(s.W, int64(l))
	s.W.Write(b)
	// o := map[string]interface{}{"type": "addplayer",
	// 	"id": player.ID, "name": player.Name}
	// s.W.Encode(o)
}

func (s *BinaryProtocol) WriteNamePlayer(player *Player) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteNamePlayer", player)
	}
	s.W.WriteByte(s.MessageMap[5])
	WriteInt32(s.W, player.ID)
	b := []byte(player.Name)
	l := len(b)
	WriteInt32(s.W, int64(l))
	s.W.Write(b)
	// o := map[string]interface{}{"type": "nameplayer",
	// 	"id": player.ID, "name": player.Name}
	// s.W.Encode(o)
}

func (s *BinaryProtocol) WriteDestroyPlayer(player *Player) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteDestroyPlayer", player)
	}
	s.W.WriteByte(s.MessageMap[6])
	WriteInt32(s.W, player.ID)
	// str := `{"type":"delplayer","id":%d}`
	// dat := fmt.Sprintf(str, player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteOwns(player *Player) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteOwns", player)
	}
	s.W.WriteByte(s.MessageMap[7])
	WriteInt32(s.W, player.ID)
	// ownsPlayer := `{"type":"own","id":%d}`
	// dat := fmt.Sprintf(ownsPlayer, player.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteDestroyActor(actor *Actor) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteDestroyActor", actor)
	}
	s.W.WriteByte(s.MessageMap[8])
	WriteInt32(s.W, actor.ID)
	// delPlayer := `{"type":"del","id":%d}`
	// dat := fmt.Sprintf(delPlayer, actor.ID)
	// _ = dat
}

func (s *BinaryProtocol) WriteMoveActor(actor *Actor) {
	if s.Logging > 1 {
		log.Println(s, "SENDING WriteMoveActor", actor)
	}
	s.W.WriteByte(s.MessageMap[9])
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.X)
	WriteFloat32(s.W, actor.Y)
	WriteFloat32(s.W, actor.Direction)
	WriteFloat32(s.W, actor.Speed)
	// delPlayer := `{"type":"move","id":%d,"x":%f,"y":%f,"d":%f,"s":%f}`
	// dat := fmt.Sprintf(delPlayer, actor.ID, actor.X, actor.Y, actor.Direction, actor.Speed)
	// _ = dat
}

func (s *BinaryProtocol) WriteSetMassActor(actor *Actor) {
	if s.Logging > 1 {
		log.Println(s, "SENDING WriteSetMassActor", actor)
	}
	s.W.WriteByte(s.MessageMap[10])
	WriteInt32(s.W, actor.ID)
	WriteFloat32(s.W, actor.Mass)
	// delPlayer := `{"type":"mass","id":%d,"mass":%f}`
	// dat := fmt.Sprintf(delPlayer, actor.ID, actor.Mass)
	// _ = dat
}

func (s *BinaryProtocol) WritePelletsIncoming(pellets []*Pellet) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WritePelletsIncoming", len(pellets))
	}

	s.W.WriteByte(s.MessageMap[11])
	WriteInt32(s.W, int64(len(pellets)))
	for _, pel := range pellets {
		WriteInt32(s.W, pel.X)
		WriteInt32(s.W, pel.Y)
		WriteInt32(s.W, pel.Type)
	}

}

func (s *BinaryProtocol) WritePlayerActor(pa *PlayerActor) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WritePlayerActor", pa)
	}

	s.W.WriteByte(s.MessageMap[12])
	s.W.WriteByte(0)
	WriteInt32(s.W, pa.Actor.ID)
	WriteInt32(s.W, pa.Player.ID)
}

func (s *BinaryProtocol) WriteVirus(v *Virus) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteVirus", v)
	}

	s.W.WriteByte(s.MessageMap[12])
	s.W.WriteByte(1)
	WriteInt32(s.W, v.Actor.ID)
}

func (s *BinaryProtocol) WriteBacteria(v *Bacteria) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteBacteria", v)
	}

	s.W.WriteByte(s.MessageMap[12])
	s.W.WriteByte(2)
	WriteInt32(s.W, v.Actor.ID)
}

func (s *BinaryProtocol) WriteBlob(v *Blob) {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteBlob", v)
	}

	s.W.WriteByte(s.MessageMap[12])
	s.W.WriteByte(3)
	WriteInt32(s.W, v.Actor.ID)
}

func (s *BinaryProtocol) WriteNewMessageMap() {
	if s.MessageMap != nil {
		s.W.WriteByte(s.MessageMap[13])
	}
	newMessages := rand.Perm(255)
	log.Println(s, "SENDING SYNC MAP SIZE", len(newMessages), newMessages)
	s.MessageMap = make([]byte, len(newMessages))
	s.W.WriteByte(byte(len(newMessages)))
	for n, om := range newMessages {
		s.MessageMap[n] = byte(om)
		s.W.WriteByte(byte(om))
	}
}

func (s *BinaryProtocol) WritePong() {
	if s.Logging > 0 {
		log.Println(s, "SENDING WritePong")
	}
	s.W.WriteByte(s.MessageMap[14])
}
func (s *BinaryProtocol) WriteStopSpectating() {
	if s.Logging > 0 {
		log.Println(s, "SENDING WriteStopSpectating")
	}
	s.W.WriteByte(s.MessageMap[15])
}

func (s *BinaryProtocol) Save() {
	s.Lock.Lock()

	a := s.W.Bytes()
	b := make([]byte, len(a))
	copy(b, a)
	s.W.Reset()

	if rand.Intn(10000) == 0 {
		log.Println(s, "RANDOMIZING BUFFER")
		s.WriteNewMessageMap()
	}

	if len(s.WriteChan) > 10 {
		log.Println(s, "HIGH WRITECHAN", len(s.WriteChan))
	}
	if len(s.WriteChan) > 50 {
		s.Lock.Unlock()
		s.Disconnected = true
		start := time.Now()
		log.Println("KILLING", s.RW)
		s.CloseChan <- fallenBehind
		log.Println("DONE CLOSING", s.RW)
		took := time.Since(start)
		if took > time.Millisecond {
			log.Println("CLOSING CONNECTION TOOK", took)
		}

	} else {
		s.WriteChan <- b
		s.Lock.Unlock()
	}
}

func (s *BinaryProtocol) SaveOob() {
	s.Lock.Lock()
	a := s.OW.Bytes()
	b := make([]byte, len(a))
	copy(b, a)
	s.OW.Reset()
	s.OobWriteChan <- b
	s.Lock.Unlock()
}

func (s *BinaryProtocol) Flush() error {
	var b []byte

	select {
	case b = <-s.OobWriteChan:
	case b = <-s.WriteChan:
	case e := <-s.CloseChan:
		return e
	}

	if s.Logging > 1 {
		log.Println(s, "FLUSHING", len(b), "BYTES")
	}
	t := time.Now()
	n, e := s.RW.Write(b)
	if e != nil {
		return e
	}
	if n < len(b) {
		log.Println("INCOMPLETE WRITE OF", n, "/", len(b))
	}
	took := time.Since(t)
	if len(b) > 5000 {
		log.Println("LONG WRITE", len(b))
	}
	if took > time.Millisecond {
		log.Println("Write took", took)
	}
	return nil
}
