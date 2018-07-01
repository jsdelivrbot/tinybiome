package tinybiome

import (
	"fmt"
	"golang.org/x/net/websocket"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"time"
)

type RoomConfig struct {
	Room *Room

	Name   string
	Width  float64
	Height float64

	MaxViruses  int
	MaxBacteria int
	MaxPellets  int

	MaxSplit        int
	MinSplitMass    float64
	MergeTime       float64
	SizeMultiplier  float64
	SpeedMultiplier float64
	StartMass       float64
}

func (c RoomConfig) String() string {
	return fmt.Sprintf("%fx%f, %dV, %dB", c.Width, c.Height, c.MaxViruses, c.MaxBacteria)
}

type NodeConfig struct {
	Name   string
	Master string

	Address  string
	Insecure bool
	Origins  []string

	Rooms []RoomConfig
}

func (nc NodeConfig) Setup(c Config, rt *RetryGroup) {
	NewServer(nc, c, rt)
}

type MasterConfig struct {
	Clients struct {
		Address  string
		Insecure bool
	}
	Management struct {
		Address  string
		Insecure bool
	}
}

func (mc MasterConfig) Run(c Config) error {
	log.Println("serving clients")
	m := http.NewServeMux()
	m.Handle("/", websocket.Handler(newConn))

	if mc.Clients.Insecure {
		if err := http.ListenAndServe(mc.Clients.Address, m); err != nil {
			log.Println("err serving clients:", err.Error())
			return err
		}
	} else {
		if err := http.ListenAndServeTLS(mc.Clients.Address, c.CertFile, c.KeyFile, m); err != nil {
			log.Println("err serving clients:", err.Error())
			return err
		}
	}
	return nil
}

func (mc MasterConfig) Manage(c Config) error {
	log.Println("serving nodes")
	m := http.NewServeMux()
	m.Handle("/", websocket.Handler(newConn))

	if mc.Management.Insecure {
		if err := http.ListenAndServe(mc.Management.Address, m); err != nil {
			log.Println("err serving nodes:", err.Error())
			return err
		}
	} else {
		if err := http.ListenAndServeTLS(mc.Management.Address, c.CertFile, c.KeyFile, m); err != nil {
			log.Println("err serving nodes:", err.Error())
			return err
		}
	}
	return nil
}

type FilesConfig struct {
	Address   string
	Insecure  bool
	Directory string
}

func (fc FilesConfig) Run(c Config) error {
	log.Println("serving files")
	w := http.NewServeMux()
	fs := NoCache(http.FileServer(http.Dir(fc.Directory)))
	w.Handle("/", fs)

	if fc.Insecure {
		runtime.SetCPUProfileRate(1000)
		runtime.SetBlockProfileRate(10)
		runtime.SetMutexProfileFraction(10)

		w.Handle("/block", pprof.Handler("block"))
		w.Handle("/profile", pprof.Handler("profile"))
		w.Handle("/heap", pprof.Handler("heap"))
		w.Handle("/mutex", pprof.Handler("mutex"))

		if err := http.ListenAndServe(fc.Address, w); err != nil {
			log.Println("err serving files: ", err.Error())
			return err
		}
	} else {
		if err := http.ListenAndServeTLS(fc.Address, c.CertFile, c.KeyFile, w); err != nil {
			log.Println("err serving files: ", err.Error())
			return err
		}
	}
	return nil
}

type Config struct {
	CertFile string
	KeyFile  string
	Master   *MasterConfig
	Files    *FilesConfig
	Nodes    []NodeConfig
}

func ConfigFromFile(fname string) (c Config, e error) {
	file, err := os.Open(fname)

	if err != nil {
		return c, err
	}

	decoder := yaml.NewDecoder(file)
	decoder.SetStrict(true)

	if err := decoder.Decode(&c); err != nil {
		return c, err
	}

	return
}

func (c Config) String() string {
	b, _ := yaml.Marshal(c)
	return string(b)
}

func (c Config) RunAndWait() error {
	rt := NewRetryGroup()
	if c.Files != nil {
		rt.Add("files", func() error {
			return c.Files.Run(c)
		})
	}
	if c.Master != nil {
		rt.Add("master.run", func() error {
			return c.Master.Run(c)
		})
		rt.Add("master.manage", func() error {
			return c.Master.Manage(c)
		})
	}
	for _, node := range c.Nodes {
		node.Setup(c, rt)
	}

	return rt.Wait()
}

var epoch = time.Unix(0, 0).Format(time.RFC1123)

var noCacheHeaders = map[string]string{
	"Expires":         epoch,
	"Cache-Control":   "no-cache, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

var etagHeaders = []string{
	"ETag",
	"If-Modified-Since",
	"If-Match",
	"If-None-Match",
	"If-Range",
	"If-Unmodified-Since",
}

func NoCache(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Delete any ETag headers that may have been set
		for _, v := range etagHeaders {
			if r.Header.Get(v) != "" {
				r.Header.Del(v)
			}
		}

		// Set our NoCache headers
		for k, v := range noCacheHeaders {
			w.Header().Set(k, v)
		}

		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
