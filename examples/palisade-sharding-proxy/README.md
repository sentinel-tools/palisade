# Palisade Example: Sharding Sentinel Proxy

This example demonstrates how to write a Palisade server which translates a
"target" into a master IP:PORT. In this scenario you connect to Palisade and
request the master for a given target just as you would for pod name. This
could be a server or user ID, for example. Palisade will return the master
information for the backend node on which that target resides.

# Scenario

Consider a sharding situation in which you have 1000 servers to track metrics
for. In this scenario each server is sharded into a group, and the group is a
dedicated Redis pod. In this scenario your metrics collection process
makes a Sentinel connection to Palisade. Palisade then runs the
consistent hashing algorithm to determine a "shard Id". This shard Id is
a pod name managed by Sentinel Constellation. Palisade then consults the
appropriate pod for the given shard Id, and returns the pod's connection
information.

# Example Setup

* Set `slotcount` to 12.
* Deploy 12 Redis pods
* Deploy a Sentinel Constellation
* Run Palisade, configured to talk to the backend sentinel constellation

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
	* Config backing stores (file, Consul)

