package util

import (
	"io/ioutil"
	"regexp"
)

const versionFilepath = "./VERSION"

var regexTrimSpaces = regexp.MustCompile(`[^\s]+`)

func Version() (string, error) {
	data, err := ioutil.ReadFile(versionFilepath)
	if err != nil {
		return "", err
	}
	return string(regexTrimSpaces.Find(data)), nil
}
