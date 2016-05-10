package main

import (
	"bufio"
	"fmt"
	"log"
	"strconv"
	"strings"
)

var (
	sentinelSubcommands map[string]CommandHandler
)

func init() {
	sentinelSubcommands = make(map[string]CommandHandler)
	sentinelSubcommands["MONITOR"] = sentinelMonitor
	sentinelSubcommands["SET"] = sentinelSet
	sentinelSubcommands["MASTER"] = sentinelGetMasterByName
	sentinelSubcommands["GET-MASTER-ADDR-BY-NAME"] = sentinelGetMasterAddressByName
}

func Sentinel(c *Command, w *bufio.Writer) error {
	subcomm := strings.ToUpper(string(c.Get(1)))
	handler, exists := sentinelSubcommands[subcomm]
	if exists {
		return handler(c, w)
	} else {
		return SendError(w, fmt.Sprintf("Command '%s' not supported", subcomm))
	}

}

func sentinelSet(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	pod, exists := pods[name]
	if !exists {
		return SendError(w, "NOSUCHPOD Pod doesn't exist")
	}
	setting := string(c.Get(3))
	value := string(c.Get(4))
	switch strings.ToUpper(setting) {
	case "AUTH-PASS":
		pod.AuthPass = value
		pods[name] = pod
		return SendOk(w)
	case "PARALLEL-SYNCS":
		nval, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			log.Printf("conversion error: '%s' doesn't become an int", value)
			return SendError(w, "INVALIDVALUE value given for parallel-syncs must be an integer")
		}
		pod.ParallelSyncs = nval
		pods[name] = pod
		return SendOk(w)
	}
	return SendError(w, fmt.Sprintf("%s is not a valid pod setting", setting))
}

func sentinelGetMasterAddressByName(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	pod, exists := pods[name]
	if !exists {
		return SendBulk(w, nil)
	}
	minfo := []string{pod.Name, pod.Port}
	return SendBulkStrings(w, minfo)
}

func sentinelGetMasterByName(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	pod, exists := pods[name]
	if !exists {
		return SendBulk(w, nil)
	}
	minfo := []string{"name", pod.Name,
		"port", pod.Port,
		"auth-pass", pod.AuthPass,
		"parallel-syncs", fmt.Sprintf("%d", pod.ParallelSyncs),
	}
	return SendBulkStrings(w, minfo)
}

func sentinelMonitor(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	ip := string(c.Get(3))
	port := string(c.Get(4))
	quorum := string(c.Get(5))
	pod := RedisPod{Name: name, IP: ip, Port: port, Quorum: quorum}
	log.Printf("Need to add pod '%s' at '%s:%s' with quorum=%s", name, ip, port, quorum)
	pods[name] = pod
	return SendOk(w)
}
