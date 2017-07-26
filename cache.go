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

func addToCache(req, msg *dns.Msg, clientIp ...string) {
	var currCache *cache.Cache
	if len(clientIp) == 0 {
		currCache = globalCache
	} else {
		addClientCache(clientIp[0])
		currCache = getClientCache(clientIp[0])
	}
	ttl := currCache.GetTTL()
	if len(msg.Answer) > 0 {
		var requestTtl = time.Duration(msg.Answer[0].Header().Ttl) * time.Second
		if requestTtl < ttl {
			ttl = requestTtl
		}
	}
	key := cache.Key(req.Question[0], false, false)
	currCache.InsertMessage(key, msg, ttl)
}

func addToGlobalCache(req, msg *dns.Msg) {
	addToCache(req, msg)
}

func addToClientSpecificCache(clientIp string, req, msg *dns.Msg) {
	addToCache(req, msg, clientIp)
}

func clearClientSpecificCaches() {
	clientSpecificCachesMutex.Lock()
	clientSpecificCaches = make(map[string]*cache.Cache)
	clientSpecificCachesMutex.Unlock()
}
