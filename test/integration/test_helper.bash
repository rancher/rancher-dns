#!/usr/bin/env bash

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )/../.." && pwd )"
DNS_BIN="$ROOT/bin/linux/amd64/rancher-dns"
DNS_CONF="$ROOT/test/fixtures/answers.json"
DNS_PORT="15353"
DIG_BIN="$(command -v dig )"
DIG_OPTS="-p $DNS_PORT +tries=1 +nosearch +noall +answer +comments +authority"

setup() {
  $DNS_BIN --listen 127.0.0.1:$DNS_PORT --answers $DNS_CONF &>/dev/null &
  pid=$!
  sleep 0.25
}

teardown() {
  pkill -P "$pid" 2>/dev/null || true
  kill -9 "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}

function resolve() {
    $DIG_BIN $DIG_OPTS @127.0.0.1 $1 $2
}

function log() {
	echo "$@"
}
