#!/usr/bin/env bats

load test_helper

@test "TTL initializes to correct value in global cache" {
  run resolve yahoo.com
  log $output
  [ $status -eq 0 ]
  yahoottl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "yahoo.com" | awk '{print $2}')
  run resolve service-baz
  log $output
  [ $status -eq 0 ]
  # cachettl=$(echo "$output" | sed -n -e 's/^.*ANSWER SECTION: service-baz. //p' | awk '{print $1}')
  cachettl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "service-baz" | awk '{print $2}')
  echo "output = ${output}"
  echo "cachettl = ${cachettl}"
  echo "yahoottl = ${yahoottl}"
  [[ "$yahoottl" -le "$cachettl" ]] || false
}

@test "TTL decreases as time passes for requests stored in client cache" {
  run resolve service-baz
  log $output
  [ $status -eq 0 ]
  old_ttl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "service-baz" | awk '{print $2}')
  for i in {1..3}; do
      sleep 5
      run resolve service-baz
      log $output
      [ $status -eq 0 ]
      new_ttl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "service-baz" | awk '{print $2}')
      echo "old_ttl = ${old_ttl}"
      echo "new_ttl = ${new_ttl}"
      [[ "$new_ttl" -lt "$old_ttl" ]] || false
      old_ttl=$new_ttl
  done
}

@test "TTL decreases as time passes for requests stored in global cache" {
  run resolve yahoo.com A
  log $output
  [ $status -eq 0 ]
  old_ttl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "yahoo.com" | awk '{print $2}')
  # old_ttl=$(dig yahoo.com | sed -n -e 's/^.*ANSWER SECTION: yahoo.com. //p' | awk '{print $1}')
  for i in {1..3}; do
      sleep 5
      run resolve yahoo.com A
      log $output
      [ $status -eq 0 ]
      # new_ttl=$(dig yahoo.com | sed -n -e 's/^.*ANSWER SECTION: yahoo.com. //p' | awk '{print $1}')
      new_ttl=$(echo "$output" | grep -A1 "ANSWER SECTION" | grep "yahoo.com" | awk '{print $2}')
      [[ "$new_ttl" -lt "$old_ttl" ]] || false
      old_ttl=$new_ttl
  done
}
