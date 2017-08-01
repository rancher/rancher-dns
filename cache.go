package main

import (
	"github.com/miekg/dns"
	"github.com/skynetservices/skydns/cache"
)

func getClientCache(clientIp string) *cache.Cache {
	clientSpecificCachesMutex.RLock()
	clientCache, ok := clientSpecificCaches[clientIp]
	clientSpecificCachesMutex.RUnlock()
	if !ok {
		clientCache = cache.New(int(*cacheCapacity), int(*defaultTtl))
		clientSpecificCachesMutex.Lock()
		clientSpecificCaches[clientIp] = clientCache
		clientSpecificCachesMutex.Unlock()
	}
	return clientCache
}

func globalCacheHit(req *dns.Msg) *dns.Msg {
	return globalCache.Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func clientSpecificCacheHit(clientIp string, req *dns.Msg) *dns.Msg {
	return getClientCache(clientIp).Hit(req.Question[0], false, false, req.MsgHdr.Id)
}

func addToGlobalCache(req, msg *dns.Msg) {
	globalCache.InsertMessage(cache.Key(req.Question[0], false, false), msg)
}

func addToClientSpecificCache(clientIp string, req, msg *dns.Msg) {
	getClientCache(clientIp).InsertMessage(cache.Key(req.Question[0], false, false), msg)
}

func clearClientSpecificCaches() {
	clientSpecificCachesMutex.Lock()
	clientSpecificCaches = make(map[string]*cache.Cache)
	clientSpecificCachesMutex.Unlock()
}
