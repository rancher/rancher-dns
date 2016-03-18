package main

import (
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns/cache"
)

func addClientCache(clientIp string) {
	if _, ok := clientSpecificCaches[clientIp]; ok {
		return
	}

	clientSpecificCaches[clientIp] = cache.New(int(*cacheCapacity), int(*defaultTtl))
}

func globalCacheHit(req *dns.Msg) *dns.Msg {
	return globalCache.Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func clientSpecificCacheHit(clientIp string, req *dns.Msg) *dns.Msg {
	addClientCache(clientIp)
	return clientSpecificCaches[clientIp].Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func addToGlobalCache(req, msg *dns.Msg) {
	key := cache.Key(req.Question[0], false, false)
	globalCache.InsertMessage(key, msg)
}

func addToClientSpecificCache(clientIp string, req, msg *dns.Msg) {
	addClientCache(clientIp)
	clientCache, _ := clientSpecificCaches[clientIp]
	key := cache.Key(req.Question[0], false, false)
	clientCache.InsertMessage(key, msg)
}

func clearClientSpecificCaches() {
	clientSpecificCaches = make(map[string]*cache.Cache)
}
