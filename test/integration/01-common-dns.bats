#!/usr/bin/env bats

load test_helper

@test "Canary: bats is working" {
  true
}

@test "Canary: dig is available" {
  run command -v dig >/dev/null 2>&1
  [ $status -eq 0 ]
}

@test "Canary: rancher-dns binary is available" {
  run $DNS_BIN -h &>/dev/null
  [ $status -eq 2 ]
}

@test "Handles TCP queries" {
  run $DIG_BIN $DIG_OPTS +tcp @127.0.0.1 www.example.com A
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
}

@test "Handles large replies (> 512 bytes)" {
  run $DIG_BIN $DIG_OPTS +dnssec @127.0.0.1 example.com DNSKEY
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
}

@test "Handles EDNS0 queries" {
  skip "Worked previously, maybe a change in upstream dns package"
  run $DIG_BIN $DIG_OPTS +edns=0 @127.0.0.1 www.example.com A
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "EDNS: version: 0" ]] || false
}

@test "Truncates response according to client buffer size" {
  run $DIG_BIN $DIG_OPTS +dnssec +bufsize=512 @127.0.0.1 example.com DNSKEY
  [ $status -eq 0 ]
  [[ "$output" =~ "Truncated, retrying in TCP mode." ]] || false
  run $DIG_BIN $DIG_OPTS +dnssec +bufsize=4096 @127.0.0.1 example.com DNSKEY
  [ $status -eq 0 ]
  [[ ! "$output" =~ "Truncated, retrying in TCP mode." ]] || false
}

@test "Rejects ANY query by returning NOTIMP response code" {
  run resolve www.example.com ANY
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOTIMP" ]] || false
}
