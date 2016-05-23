package main

type SentinelSet map[string]struct{}

func NewConstellation() SentinelSet {
	newset := make(SentinelSet)
	return newset
}

func (c *SentinelSet) Add(address string) bool {
	_, exists := (*c)[address]
	(*c)[address] = struct{}{}
	return exists
}

func (c *SentinelSet) Remove(address string) {
	delete((*c), address)
}

func (c *SentinelSet) Contains(address string) bool {
	_, exists := (*c)[address]
	return exists
}
