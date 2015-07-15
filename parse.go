package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

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
	return out, err
}
