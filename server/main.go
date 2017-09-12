package main


import (
	"log"
	"os"
	"runtime"
	"encoding/json"

	"app/shared/jsonconfig"
	"app/websock"

	"app/route"
	"time"

)

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
)


func init(){
	log.SetFlags(log.Lshortfile)
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main(){
	jsonconfig.Load("config"+string(os.PathSeparator)+"config.json", config)

	websock.Run(route.LoadHTTP(), config.Server)
}

var config = &configuration{}

type configuration struct {
	//Database  database.Info   `json:"Database"`
	Server    websock.Server   `json:"Server"`
}

func (c *configuration) ParseJSON(b []byte) error {
	return json.Unmarshal(b, &c)
}
