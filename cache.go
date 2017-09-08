package main

import (
	"time"

	"github.com/miekg/dns"
	"github.com/rancher/rancher-dns/cache"
)

func getClientCache(clientUUID string) *cache.Cache {
	clientSpecificCachesMutex.RLock()
	clientCache, ok := clientSpecificCaches[clientUUID]
	clientSpecificCachesMutex.RUnlock()
	if !ok {
		clientCache = cache.New(int(*cacheCapacity), int(*defaultTtl))
		clientSpecificCachesMutex.Lock()
		clientSpecificCaches[clientUUID] = clientCache
		clientSpecificCachesMutex.Unlock()
	}
	return clientCache
}

func globalCacheHit(req *dns.Msg) (*dns.Msg, time.Time) {
	return globalCache.Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func clientSpecificCacheHit(clientUUID string, req *dns.Msg) (*dns.Msg, time.Time) {
	return getClientCache(clientUUID).Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func addToCache(req, msg *dns.Msg, clientUUID ...string) {
	var currCache *cache.Cache
	if len(clientUUID) == 0 {
		currCache = globalCache
	} else {
		currCache = getClientCache(clientUUID[0])
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

func addToClientSpecificCache(clientUUID string, req, msg *dns.Msg) {
	addToCache(req, msg, clientUUID)
}

func clearClientSpecificCaches() {
	clientSpecificCachesMutex.Lock()
	clientSpecificCaches = make(map[string]*cache.Cache)
	clientSpecificCachesMutex.Unlock()
}
