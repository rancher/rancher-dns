// +build darwin freebsd linux netbsd openbsd

package main

import (
	"bufio"
	"os"
	"strings"
)

func getGlobalRecurse() ([]string, error) {
	var recurse []string
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return recurse, err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "nameserver") {
			dns := strings.TrimSpace(l[11:len(l)])
			if invalidRecurse(dns) {
				continue
			}
			recurse = append(recurse, dns)
		}
	}

	if len(recurse) == 0 {
		return fallbackRecurse, nil
	}

	return recurse, nil
}
