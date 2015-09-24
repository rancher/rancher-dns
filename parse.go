package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
)

func ParseAnswers(path string) (out Answers, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn("Failed to find: ", path)
		}
		return nil, err
	}

	out = make(Answers)
	err = json.Unmarshal(data, &out)
	if err == nil {
		ConvertPtrIps(&out)
		return out, nil
	} else {
		return nil, err
	}
}

func ConvertPtrIps(answers *Answers) {
	// Convert PTR keys that are IP addresses into "4.3.2.1.in-addr.arpa." form.
	for _, client := range *answers {
		for origKey, val := range client.Ptr {
			if !strings.HasSuffix(origKey, "in-addr.arpa.") {
				newKey := "in-addr.arpa."
				for _, i := range strings.Split(origKey, ".") {
					newKey = i + "." + newKey
				}

				delete(client.Ptr, origKey)
				client.Ptr[newKey] = val
				log.Debug("Transformed PTR for ", origKey, " to ", newKey, " => ", val.Answer)
			}
		}
	}
}
