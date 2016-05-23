# Palisade Example: Sentinel Authentication Proxy

This example implements a subset of sentinel commands suitable for obtaining
master information for a given Redis pod via Palisade serving as an
authenticating proxy. This would be useful for implementing an authenticated
sentinel for clients to conenct to.

In its current form it is very basic, being an example and all. 


# Terminology

Managing Sentinel: an IP:PORT string representing an actual sentinel whch manages the pod(s).

# Caveats

* You can pass the following commands which are proxied to the mangaing sentinels:
	* sentinel master <podname>
	* sentinel get-master-add-by-name

A significant caveat in my mind is that the commands are proxied to the first
sentinel to respond. What I'd want to see is for it to query each sentinel
known by the pod and pass along the information from QUORUM/KNOWN sentinels to
help ensure you get accurate information.

# Command Options

You can use `-h` or `help`, or `--help` to get comand line help.

## Setting Port
Command flag: `-p`or  `--port` followed by the port number.

## Setting Auth token
Command flag: `-a`or  `--authtoken` followed by the token to use

## Setting Sentinel addresses
Command flag: `-s` or `--sentineladdr` followed by the IP:PORT string
identifying the address.  For this command you can pass it for each sentinel
to add to the proxy's pool.


# TODO
	* proxy slaves command 
	* Config backing stores (file, Consul)

# Strech TODOs
	* implement command to specify pod's auth token
	* smarter sentinel proxying (see caveats)
