package collector

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.org/x/oauth2/google"
)

func GetGCPZoneFromURL(logger log.Logger, z string) string {
	u, err := url.Parse(z)
	if err != nil {

		level.Error(logger).Log("msg", "error parsing Zone name", "err", err)
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

var (
	GCPHttpTimeout        time.Duration
	GCPMaxRetries         int
	GCPRetryStatuses      []int
	GCPBackoffJitterBase  time.Duration
	GCPMaxBackoffDuration time.Duration
)

func NewGCPClient(ctx context.Context, scope string) (client *http.Client, err error) {
	googleClient, err := google.DefaultClient(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("error creating Google client: %+v", err)
	}

	googleClient.Timeout = GCPHttpTimeout
	googleClient.Transport = rehttp.NewTransport(
		googleClient.Transport,
		rehttp.RetryAll(
			rehttp.RetryMaxRetries(GCPMaxRetries),
			rehttp.RetryStatuses(GCPRetryStatuses...)), // Cloud support suggests retrying on 503 errors
		rehttp.ExpJitterDelay(GCPBackoffJitterBase, GCPMaxBackoffDuration), // Set timeout to <10s as that is Prometheus' default timeout
	)
	return googleClient, nil
}
