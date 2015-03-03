rancher-dns
===========

A simple DNS server that returns different answers depending on the IP address of the client making the request.

# Usage
```bash
  rancher-dns [--debug] [--listen host:port] [--ttl num] --answers /path/to/answers.json
```

# Compile
```
  godep go build
```

## CLI Options

Option      | Default        | Description
------------|----------------|------------
`--debug`   | *off*          | If present, more debug info is logged
`--listen`  | 0.0.0.0:53     | IP address and port to listen on (TCP &amp; UDP)
`--answers` | ./answers.json | Path to a JSON file with client-specific answers
`--ttl`     | 600            | TTL for local (non-recursive) responses that are returned

## JSON Answers File
The answers file should be a JSON map with a key for each client IP address.
  - Each key is a map of FQDN to an array of answers.
  - Top-level keys should be IP addresses
    - A special key `"default"` will be checked if no IP address match is found
  - 2nd-level keys should be *fully qualified*, ending in a dot.
    - A special key `"recurse"` can contain a list of servers to make a recursive query to if no answer is found locally.

```javascript
{
  "default": {
    "recurse": ["8.8.8.8"],
    "rancher.com.": ["1.2.3.4"]
  },
  "10.1.2.2": {
    "mysql.": ["10.1.2.3"],
    "web.": ["10.1.2.4","10.1.2.5","10.1.2.6"]
  },
  "192.168.0.2": {
    "recurse": ["8.8.4.4:53","8.8.8.8"],
    "mysql.": ["192.168.0.3"],
    "web.": ["192.168.0.4","192.168.0.5","192.168.0.6"]
  }
}
```

## Answering queries
A query is answered by returning the first match of:
  - An entry in the answers map for the client's IP.
  - An entry in the answers map in the `"default"` key.
  - If there is a `"recurse"` key for the client's IP, perform recursive lookup on each of those servers (in order).
  - If there is a `"recurse"` key for the `"default"`, perform recursive lookup on each of those servers (in order).
  - Do not pass go, do not collect $200.  Return `SERVFAIL`.

## Limitations
  - Only A records are currently supported.
