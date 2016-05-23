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
	"log"
	"net"
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
)

func init() {
	commandHandlers = make(map[string]CommandHandler)
	commandHandlers["SENTINEL"] = Sentinel
	commandHandlers["AUTH"] = authConnection
	commandHandlers["ADDSENTINEL"] = addSentinel
	commandHandlers["KNOWNSENTINELS"] = knownSentinels
	stockData = make(map[string][]byte)
	stockData["foo"] = []byte{'f', 'o', 'o'}
	pods = make(map[string]RedisPod)
	tokens = make(map[string]bool)
	tokens["secretpass1"] = true
	managingSentinels = NewConstellation()
}

func main() {
	listener, err := net.Listen("tcp", ":6380")
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
