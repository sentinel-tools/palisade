package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"strings"

	"github.com/fatih/structs"
	"github.com/therealbill/libredis/client"
)

var (
	sentinelSubcommands map[string]CommandHandler
	slotcount           float64
)

func init() {
	sentinelSubcommands = make(map[string]CommandHandler)
	sentinelSubcommands["MASTER"] = sentinelGetMasterByName
	sentinelSubcommands["GET-MASTER-ADDR-BY-NAME"] = sentinelGetMasterAddressByName
	// make configurable?
	slotcount = 4
}

func getShardId(key string) (string, error) {
	h := fnv.New32a()
	h.Write([]byte(key))
	num := float64(h.Sum32())
	slotnum := math.Mod(num, slotcount)
	log.Print(key, "->", slotnum)
	return fmt.Sprintf("shard-%d", int(slotnum)), nil
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

func sentinelGetMasterAddressByName(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	shardid, _ := getShardId(name)
	log.Print(shardid)
	var minfo []string
	for sa, _ := range managingSentinels {
		sc, err := client.DialAddress(sa)
		if err == nil {
			res, _ := sc.SentinelGetMaster(shardid)
			if res.Host > "" {
				minfo = append(minfo, res.Host)
				minfo = append(minfo, fmt.Sprintf("%d", res.Port))
				return SendBulkStrings(w, minfo)
			}
			log.Printf("[%s] no such pod", sa)
			continue
		}
		log.Printf("[%s] error: %s", sa, err.Error())
	}
	log.Printf("Shard for target '%s' not found anywhere, return error", name)
	return SendError(w, fmt.Sprintf("-ERR No target for '%s'", name))
}

func sentinelGetMasterByName(c *Command, w *bufio.Writer) error {
	name := string(c.Get(2))
	shardid, _ := getShardId(name)
	var minfo []string
	for sa, _ := range managingSentinels {
		sc, err := client.DialAddress(sa)
		if err == nil {
			res, err := sc.SentinelMaster(shardid)
			if res.Name > "" {
				m := structs.Fields(res)
				for _, v := range m {
					minfo = append(minfo, v.Tag("redis"))
					minfo = append(minfo, fmt.Sprintf("%v", v.Value()))
				}
			}
			if err != nil {
				log.Printf("[%s] error: %s", sc, err.Error())
			}
		}
	}
	return SendBulkStrings(w, minfo)
}
