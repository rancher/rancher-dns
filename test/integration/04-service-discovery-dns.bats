#!/usr/bin/env bats

load test_helper

@test "Returns NXDOMAIN for query referencing non-existing service" {
  run resolve no-service.no-stack.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NXDOMAIN" ]] || false
}

@test "Returns NODATA for AAAA query referencing existing service" {
  run resolve service-foo.stack-a.rancher.internal AAAA
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 0," ]] || false
}

@test "Returns NODATA for AAAA query referencing existing service (non-fully qualified)" {
  run resolve service-foo.stack-a AAAA
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 0," ]] || false
}

# Services

@test "Query for service is resolved (<service>.<stack>.rancher.internal)" {
  run resolve service-bar.stack-b.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.112.11" ]] || false
}

@test "Query for service in the same stack is resolved (<service>)" {
  run resolve service-baz A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 2," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.141.123" ]] || false
  [[ "$output" =~ IN.*A.*"10.42.195.8" ]] || false
}

@test "Query for service in other stack is resolved (<service>.<stack>)" {
  run resolve service-bar.stack-b A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.112.11" ]] || false
}

# Containers

@test "Query for container is resolved (<container>.rancher.internal)" {
  run resolve stack-a_service-baz_1.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.141.123" ]] || false
}

@test "Query for container is resolved (<container>)" {
  run resolve stack-a_service-baz_1 A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.141.123" ]] || false
}

# Sidekicks

@test "Query for sidekick is resolved (<sidekick>.<service>.<stack>.rancher.internal)" {
  run resolve sidekick-bar.service-foo.stack-a.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.209.131" ]] || false
}

@test "Query for sidekick in the same service is resolved (<sidekick>)" {
  run resolve sidekick-bar A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.209.131" ]] || false
}

@test "Query for sidekick in the same stack is resolved (<sidekick>.<service>)" {
  run resolve sidekick-bar.service-foo A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.209.131" ]] || false
}

@test "Query for sidekick in other stack is resolved (<sidekick>.<service>.<stack>)" {
  run resolve sidekick-baz.service-bar.stack-b A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ "ANSWER: 1," ]] || false
  [[ "$output" =~ IN.*A.*"10.42.209.120" ]] || false
}

# External Alias

@test "Query for external alias is resolved (<external-alias>.<stack>.rancher.internal)" {
  run resolve external-alias-foo.stack-a.rancher.internal A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ IN.*A.*"93.184.216.34" ]] || false
}

@test "Query for external alias in the same stack is resolved (<external-alias>)" {
  run resolve external-alias-foo A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ IN.*A.*"93.184.216.34" ]] || false
}

@test "Query for external alias in other stack is resolved (<external-alias>.<stack>)" {
  run resolve external-alias-bar.stack-b A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ IN.*A.*"93.184.216.34" ]] || false
}

# Metadata

@test "Query for Rancher Metadata is resolved" {
  run resolve rancher-metadata A
  log $output
  [ $status -eq 0 ]
  [[ "$output" =~ "status: NOERROR" ]] || false
  [[ "$output" =~ IN.*A.*"169.254.169.250" ]] || false
}
