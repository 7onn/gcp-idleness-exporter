package main

import (
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

func GetGCPZoneFromURL(z string) string {
	u, err := url.Parse(z)
	if err != nil {
		log.Error().Msgf("error parsing Zone name, %+v", err)
	}

	parts := strings.Split(u.Path, "/")

	var zone string
	for i := 0; i < len(parts); i++ {
		if parts[i] == "zones" {
			zone = parts[i+1]
			i++
		}
	}

	return zone
}
