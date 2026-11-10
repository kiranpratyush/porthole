package utils

import (
	"errors"
	"strings"
)

func ParseClientName(host string) (string, error) {
	domainsplits := strings.Split(host, ".")
	if len(domainsplits) < 2 {
		return "", errors.New("No subdomain found in host")
	}
	return domainsplits[0], nil
}
