/*
Big todo:
 use a CLI/ENV route to provide sentinel name and Consul information. This is
 then used to connect to Consul to pull down configuration as well as storage
 and retrieval of pod information. Eventually it could also be used for leader
 elections and failover management. Events would also be published to Consul.

 One big advantage of the Consul integration is that you can integrate
 consul-template to manage clients such as a TCP proxy or for client
 reconfiguration.


 Example Uses
 1. As a single-contact point for sentinel management tools.
	Many pod actions have to be taken on every sentinel. You could adapt this
	code to do that for you if it knows of a bank of sentinels or it could
	contact Redskull to do these things and let Redskull do that work. Indeed
	this code will be integrated into Redskull so you could talk to Redskull as
	if it were a sentinel.
 2. For custom service discovery
	You could connect this to a database which stored names, and passwords, for
	Redis instances not actually managed by Sentinel. With this you could
	provide a Sentinel interface so clients don't need to do custom
	configuration.
 3. Hashing Discovery Service Another aspect of custom service discovery would
	be to have the sentinel get-master-addr command actually run a hashing
	algorithm to determine what server to conenct to and return that. In this mode
	you would use the key as the pod name.
 4. Mocks for testing
	Because you control what this code does, you can customize the behavior in
	order to do sentinel integration testing. For example you could have
	interactions with a pod named 'fail-pod' always return an error. You could
	have a random error condition such as every other command results in a
	disconnection.  By doing this you can have your sentinel interacting code
	known how to handle specific errors and be able to test said code.
 5. Sentinel auth proxy
 	Sentinel doesn't handle auth. With this as a proxy you could easily have
	a sentinel whoch handles authentication, even does ACLs, and have it proxy
	actual sentinel commands to the backend sentinel server(s).
*/
package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
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
	stockData       map[string][]byte
	commandHandlers map[string]CommandHandler
	pods            map[string]RedisPod
	tokens          map[string]bool
)

func init() {
	commandHandlers = make(map[string]CommandHandler)
	commandHandlers["GET"] = Get
	commandHandlers["SET"] = Set
	commandHandlers["SENTINEL"] = Sentinel
	commandHandlers["AUTH"] = authConnection
	stockData = make(map[string][]byte)
	stockData["foo"] = []byte{'f', 'o', 'o'}
	pods = make(map[string]RedisPod)
	tokens = make(map[string]bool)
	tokens["secretpass1"] = true
}

func authConnection(c *Command, w *bufio.Writer) error {
	token := string(c.Get(1))
	valid, exists := tokens[token]
	if exists && valid {
		return nil
	}
	return errors.New("Invalid auth")
}

func Set(command *Command, w *bufio.Writer) error {
	log.Print("SET called")
	key := string(command.Get(1))
	value := command.Get(2)
	stockData[key] = value
	log.Printf("value set for %s", key)
	return SendOk(w)
}

func Get(command *Command, w *bufio.Writer) error {
	key := string(command.Get(1))
	value, exists := stockData[key]
	if exists {
		return SendString(w, string(value))
	}
	return SendBulk(w, nil)
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	parser := NewParser(conn)
	w := bufio.NewWriter(conn)
	authorized := false
	authfails := 0
	maxauths := 3
	unauthedCommands := 0
	maxUnauthCommands := 3
	var ew error
	for {
		command, err := parser.ReadCommand()
		if err != nil {
			_, ok := err.(*ProtocolError)
			if ok {
				ew = SendError(w, err.Error())
			} else {
				log.Println(conn.RemoteAddr(), "closed connection")
				break
			}
		} else {
			cmd := strings.ToUpper(string(command.Get(0)))
			if cmd == "QUIT" {
				conn.Close()
				break
			}
			if !authorized {
				if cmd == "AUTH" {
					handler, exists := commandHandlers[cmd]
					if !exists {
						ew = SendError(w, "AUTH Command not supported")
						continue
					}
					ew = handler(command, w)
					if ew != nil {
						authfails++
						if authfails == maxauths {
							SendError(w, "GOAWAY Too many failed auth attempts")
							log.Printf("Connection terminated for %s due to too many failed auth attempts", conn.RemoteAddr())
							break
						}
						ew = SendError(w, "INVALIDAUTH Need to auth first")
						continue
					}
					authorized = true
					log.Printf("Client %s authorized successfully", conn.RemoteAddr())
					ew = SendOk(w)
					if ew != nil {
						log.Printf("Error on send: %v", ew)
						break
					} else {
						log.Print("OK sent")
					}
					continue

				} else {
					unauthedCommands++
					if unauthedCommands == maxUnauthCommands {
						SendError(w, "GOAWAY Too many unauthenticated commands")
						log.Printf("Connection terminated for %s due to too many unauthed command attempts", conn.RemoteAddr())
						break
					}
					ew = SendError(w, "NOVALIDAUTH Need to auth first")
					continue
				}
			}
			handler, exists := commandHandlers[cmd]
			if exists {
				ew = handler(command, w)
			} else {
				log.Printf("unsupported command: %s", cmd)
				var args []string
				for x := 1; x <= command.ArgCount(); x++ {
					args = append(args, string(command.Get(x)))
				}
				log.Printf("Unsupported command: '%s' with args: '%+v'", string(command.Get(1)), args)
				ew = SendError(w, fmt.Sprintf("Command '%s' not supported", cmd))
				break
			}
		}
		if ew != nil {
			log.Println("ew: ", ew)
			break
		}
	}
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
