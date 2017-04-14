#!/usr/bin/env bats

load test_helper

@test "Sets RA flag in authoritative response when recursion requested" {
  skip "Needs to be fixed"
  run resolve x.rancher.internal A
  log $output
  [ $status -eq 0 ]
  log $output
  [[ "$output" =~ "rd ra; QUERY" ]] || false
}

@test "Sets AA flag in authoritative response" {
  run resolve x.rancher.internal A
  log $output
  [ $status -eq 0 ]
  echo "Got: $output"
  [[ "$output" =~ "flags: qr aa" ]] || false
}

# RFC 2308
@test "Returns NODATA response when name is valid but there are no records of the given type" {
  run resolve service-foo.stack-a.rancher.internal AAAA
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 0," ]] || false
}

# RFC 2308
@test "NODATA response contains the SOA record for the authoritative domain" {
  skip "Needs to be fixed"
  run resolve service-foo.stack-a.rancher.internal AAAA
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "AUTHORITY: 1," ]] || false
  [[ "$output" =~ rancher.internal.*IN.*SOA ]] || false
}

# RFC 1035
@test "Returns NXDOMAIN response when the name does not exist" {
  run resolve nonexisting.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NXDOMAIN" ]] || false
}

# RFC 2308
@test "NXDOMAIN response contains the SOA record for the authoritative domain" {
  run resolve nonexisting.rancher.internal AAAA
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "AUTHORITY: 1," ]] || false
  [[ "$output" =~ rancher.internal.*IN.*SOA ]] || false
}

@test "Handles very long record name" {
  name=ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd.ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.rancher.internal
  run resolve $name A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ $name.*IN.*A.*169.254.169.250 ]] || false
}

@test "Response to query matching a CNAME contains the CNAME record and the target record" {
  run resolve external-alias-foo.stack-a.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 2," ]] || false
  [[ "$output" =~ IN.*CNAME.*"www.example.com." ]] || false
  [[ "$output" =~ IN.*A.*"93.184.216.34" ]] || false
}
