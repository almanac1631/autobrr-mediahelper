package autobrrmediahelper

import "time"

type Config struct {
	DBName              string
	WebserverPort       int
	AuthorizationValue  []byte
	MediaScrapeInterval time.Duration
}
