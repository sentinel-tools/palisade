/*
Sentinel auth proxy example
Sentinel doesn't handle auth. With this as a proxy you
could easily have usable authentication followed by the actual sentinel
commands being proxied to the managing sentinel server(s).
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

func authConnection(c *Command, w *bufio.Writer) error {
	token := string(c.Get(1))
	valid, exists := tokens[token]
	if exists && valid {
		return nil
	}
	return errors.New("Invalid auth")
}

func addSentinel(c *Command, w *bufio.Writer) error {
	sa := string(c.Get(1))
	managingSentinels.Add(sa)
	return SendOk(w)
}

func knownSentinels(c *Command, w *bufio.Writer) error {
	var sentinels []string
	for s, _ := range managingSentinels {
		sentinels = append(sentinels, s)
	}
	return SendBulkStrings(w, sentinels)
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
