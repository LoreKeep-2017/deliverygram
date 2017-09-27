package websock

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"os"
	"flag"
	"encoding/json"
	"io/ioutil"
	"app/store/types"
	"app/store"
)

type Server struct {
	Hostname  string `json:"Hostname"`
	UseHTTP   bool   `json:"UseHTTP"`
	UseHTTPS  bool   `json:"UseHTTPS"`
	HTTPPort  int    `json:"HTTPPort"`
	HTTPSPort int    `json:"HTTPSPort"`
	CertFile  string `json:"CertFile"`
	KeyFile   string `json:"KeyFile"`
	Pool 	  int    `json:"Pool"`
}

var globals struct {
	hub           *Hub
	sessionStore  *SessionStore
	//cluster       *websock.Cluster
	apiKeySalt    []byte
	indexableTags []string
	// Add Strict-Transport-Security to headers, the value signifies age.
	// Empty string "" turns it off
	tlsStrictMaxAge string
}

const (
	// Terminate session after this timeout.
	IDLETIMEOUT = time.Second * 55
	// Keep topic alive after the last session detached.
	TOPICTIMEOUT = time.Second * 5

	// Current API version
	VERSION = "0.13"
	// Minimum supported API version
	MIN_SUPPORTED_VERSION = "0.13"

	// TODO: Move to config
	DEFAULT_GROUP_AUTH_ACCESS = types.ModeCPublic
	DEFAULT_P2P_AUTH_ACCESS   = types.ModeCP2P
	DEFAULT_GROUP_ANON_ACCESS = types.ModeNone
	DEFAULT_P2P_ANON_ACCESS   = types.ModeNone
)


type configType struct {
	// Default port to listen on. Either numeric or canonical name, e.g. :80 or :https
	// Could be blank: if TLS is not configured, will use port 80, otherwise 443
	// Can be overriden from the command line, see option --listen
	Listen string `json:"listen"`
	// Salt used in signing API keys
	APIKeySalt []byte `json:"api_key_salt"`
	// Tags allowed in index (user discovery)
	IndexableTags []string                   `json:"indexable_tags"`
	ClusterConfig json.RawMessage            `json:"cluster_config"`
	StoreConfig   json.RawMessage            `json:"store_config"`
	PushConfig    json.RawMessage            `json:"push"`
	TlsConfig     json.RawMessage            `json:"tls"`
	AuthConfig    map[string]json.RawMessage `json:"auth_config"`
}

func Run(httpHandlers http.Handler, s Server) {
	startHTTP(httpHandlers, s)
}

func startHTTP(handlers http.Handler, s Server) {
	var configfile = flag.String("config", "./config/server.conf", "Path to config file.")
	var listenOn = flag.String("listen", "", "Override TCP address and port to listen on.")
	var staticPath = flag.String("static_data", "", "Path to /static data for the server.")
	var tlsEnabled = flag.Bool("tls_enabled", false, "Override config value for enabling TLS")

	var config configType
	if raw, err := ioutil.ReadFile(*configfile); err != nil {
		log.Fatal(err)
	} else if err = json.Unmarshal(raw, &config); err != nil {
		log.Fatal(err)
	}

	if *listenOn != "" {
		config.Listen = *listenOn
	}

	var err = store.Open(string(config.StoreConfig))
	if err != nil {
		log.Fatal("Failed to connect to DB: ", err)
	}
	defer func() {
		store.Close()
		log.Println("Closed database connection(s)")
	}()

	//err = push.Init(string(config.PushConfig))
	//if err != nil {
	//	log.Fatal("Failed to initialize push notifications: ", err)
	//}
	//defer func() {
	//	push.Stop()
	//	log.Println("Stopped push notifications")
	//}()


	// Keep inactive LP sessions for 15 seconds
	globals.sessionStore = NewSessionStore(IDLETIMEOUT + 15*time.Second)
	// The hub (the main message router)
	globals.hub = newHub()
	// Cluster initialization
	//clusterInit(config.ClusterConfig)
	// API key validation secret
	globals.apiKeySalt = config.APIKeySalt
	// Indexable tags for user discovery
	globals.indexableTags = config.IndexableTags

	var staticContent = *staticPath
	if staticContent == "" {
		path, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
		staticContent = path + "/static/"
	}

	http.Handle("/x/", http.StripPrefix("/x/", hstsHandler(http.FileServer(http.Dir(staticContent)))))
	//log.Printf("Serving static content from '%s'", staticContent)


	fmt.Println(time.Now().Format("2006-01-02 03:04r:05 PM"), "Running HTTP "+httpAddress(s))

	http.HandleFunc("/api/ws", ServerWebSocket)


	if err := listenAndServe(config.Listen, *tlsEnabled, string(config.TlsConfig), signalHandler()); err != nil {
		log.Fatal(err)
	}

	//log.Fatal(http.ListenAndServe(httpAddress(s), handlers))
}

func httpAddress(s Server) string {
	return s.Hostname + ":" + fmt.Sprintf("%d", s.HTTPPort)
}

