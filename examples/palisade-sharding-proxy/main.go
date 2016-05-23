/*
Sentinel auth proxy Sentinel doesn't handle auth. With this as a proxy you
could easily have usable authentication followed by the actual sentinel
commands being proxied to the managing sentinel server(s).

# Ways to improve this example
 * Subscribe to the Sentinel event channels to catch sdown events on
   sentinels, using these to update the constellation in real time.
 * Use discovery to populate Constellation information from a single sentinel
 * Config backing store such as Consul
*/

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/codegangsta/cli"
)

type CommandHandler func(*Command, *bufio.Writer) error
type RedisPod struct {
	Name          string
	IP            string
	Port          string
	Quorum        string
	AuthPass      string
	ParallelSyncs int64
}

var (
	stockData         map[string][]byte
	commandHandlers   map[string]CommandHandler
	pods              map[string]RedisPod
	tokens            map[string]bool
	managingSentinels SentinelSet
	app               *cli.App
)

func init() {
	commandHandlers = make(map[string]CommandHandler)
	commandHandlers["SENTINEL"] = Sentinel
	commandHandlers["AUTH"] = authConnection
	commandHandlers["ADDSENTINEL"] = addSentinel
	commandHandlers["KNOWNSENTINELS"] = knownSentinels
	tokens = make(map[string]bool)
	//tokens["secretpass1"] = true
	managingSentinels = NewConstellation()
}

func main() {

	app = cli.NewApp()
	app.Name = "palisade-auth-proxy"
	app.Usage = "An (example) authentication proxy for Sentinel"
	app.Version = "0.2"
	app.Authors = append(app.Authors, cli.Author{Name: "Bill Anderson", Email: "therealbill@me.com"})
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "authtoken, a",
			Value:  "secretpass1",
			Usage:  "The auth token to use for palisade",
			EnvVar: "PALISADE_AUTH",
		},
		cli.IntFlag{
			Name:   "port, p",
			Value:  26380,
			Usage:  "The port to listen on",
			EnvVar: "PALISADE_PORT",
		},
		cli.StringSliceFlag{
			Name:   "sentineladdr,s",
			EnvVar: "PALISADE_MANAGINGSENTINELS",
		},
	}

	app.Action = serve
	app.Run(os.Args)
}

func serve(c *cli.Context) {
	port := c.Int("port")
	auth := c.String("authtoken")

	tokens[auth] = true
	for _, sa := range c.StringSlice("sentineladdr") {
		log.Printf("adding managing sentinel %s", sa)
		managingSentinels.Add(sa)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error on accept: ", err)
			continue
		}
		go handleConnection(conn)
	}
}
