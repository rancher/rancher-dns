package main

import (
	"time"

	"github.com/miekg/dns"
	"github.com/rancher/rancher-dns/cache"
)

func getClientCache(clientIp string) *cache.Cache {
	clientSpecificCachesMutex.RLock()
	cache, ok := clientSpecificCaches[clientIp]
	clientSpecificCachesMutex.RUnlock()

	if ok {
		return cache
	}

	return nil
}

func addClientCache(clientIp string) {
	if clientCache := getClientCache(clientIp); clientCache != nil {
		return
	}

	clientSpecificCachesMutex.Lock()
	clientSpecificCaches[clientIp] = cache.New(int(*cacheCapacity), int(*defaultTtl))
	clientSpecificCachesMutex.Unlock()
}

func globalCacheHit(req *dns.Msg) (*dns.Msg, time.Time) {
	return globalCache.Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func clientSpecificCacheHit(clientIp string, req *dns.Msg) (*dns.Msg, time.Time) {
	addClientCache(clientIp)
	clientCache := getClientCache(clientIp)
	return clientCache.Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func addToGlobalCache(req, msg *dns.Msg) {
	key := cache.Key(req.Question[0], false, false)

	var globalTtl = globalCache.GetTTL()
	var requestTtl = globalTtl

	if len(msg.Answer) > 0 {
		requestTtl = time.Duration(msg.Answer[0].Header().Ttl) * time.Second
	}
	if requestTtl < globalTtl {
		globalCache.InsertMessage(key, msg, requestTtl)
	} else {
		globalCache.InsertMessage(key, msg, globalTtl)
	}
}

func addToClientSpecificCache(clientIp string, req, msg *dns.Msg) {
	addClientCache(clientIp)
	clientCache := getClientCache(clientIp)
	key := cache.Key(req.Question[0], false, false)
	clientCache.InsertMessage(key, msg, clientCache.GetTTL())
}

func clearClientSpecificCaches() {
	clientSpecificCachesMutex.Lock()
	clientSpecificCaches = make(map[string]*cache.Cache)
	clientSpecificCachesMutex.Unlock()
}
