#!/usr/bin/env bats

load test_helper

@test "Recursive queries (A)" {
  # NOERROR
  run resolve www.example.com A
  log $output
  [ "$status" -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "A	93.184.216.34" ]] || false

  # NXDOMAIN
  run resolve subdomain.invalid A
  log $output
  [ "$status" -eq 0 ]
  [[ "$output" =~ "status: NXDOMAIN" ]] || false
}

@test "Recursive queries (AAAA)" {
  skip "Needs to be fixed"
  # NOERROR
  run resolve www.example.com AAAA
  log $output
  [ "$status" -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "AAAA	2606:2800:220:1:248:1893:25c8:1946" ]] || false

  # NXDOMAIN
  run resolve subdomain.invalid AAAA
  log $output
  [ "$status" -eq 0 ]
  [[ "$output" =~ "status: NXDOMAIN" ]] || false
}

@test "Recursive queries (TXT/MX)" {
  run resolve example.com TXT
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false

  run resolve example.com MX
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
}

@test "Sets RA flag in recursive response when recursion requested" {
  run resolve www.example.com A
  [ $status -eq 0 ]
  log $output
  [[ "$output" =~ " rd ra " ]] || false

  run resolve subdomain.invalid A
  log $output
  [[ "$output" =~ " rd ra " ]] || false
}
