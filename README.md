rancher-dns
===========

A simple DNS server that returns different answers depending on the IP address of the client making the request.

# Usage
```bash
  rancher-dns [--debug] \
    [--listen host:port] \
    --recurse host[:port] \
    --answers /path/to/answers.json \
    [--ttl num]
```

# Compile
```
  godep go build
```

## CLI Options

Option      | Default        | Description
------------|----------------|------------
`--debug`   | *off*          | If present, more debug info is logged
`--listen`  | 0.0.0.0:53     | IP address and port to listen on
`--recurse` | *none*         | Host and port to send recursive queries to
`--answers` | ./answers.json | Path to a JSON file with client-specific answers
`--ttl`     | 600            | TTL for non-recursive responses that are returned

## JSON Answers File
The answers file should be a JSON map with a key for each client IP address.  Each key is a map of FQDN to an array of answers.  If an answer is not found in the map for the client's IP, the `"default"` key will be checked for a match.

```javascript
{
  "default": {
    "rancher.com.": ["1.2.3.4"]
  },
  "10.1.2.2": {
    "mysql.": ["10.1.2.3"],
    "web.": ["10.1.2.4","10.1.2.5","10.1.2.6"]
  },
  "192.168.0.2": {
    "mysql.": ["192.168.0.3"],
    "web.": ["192.168.0.4","192.168.0.5","192.168.0.6"]
  }
}
```

## Limitations
  - Only A records are currently supported.
