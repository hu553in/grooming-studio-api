package avito

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
)

const (
	firefoxTemplate   = "Mozilla/5.0 (%s; rv:%.1f) Gecko/20100101 Firefox/%.1f"
	chromeTemplate    = "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36"
	fallbackUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10; rv:58.0) Gecko/20100101 Firefox/58.0"
)

func nextUserAgent(ctx context.Context, log *slog.Logger) string {
	var uaGens = []func(ctx context.Context, log *slog.Logger) string{
		genFirefoxUserAgent,
		genChromeUserAgent,
	}
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(uaGens))))
	if err != nil {
		log.ErrorContext(ctx, "failed to generate random user agent", "error", err)
		return fallbackUserAgent
	}
	return uaGens[idx.Int64()](ctx, log)
}

func genFirefoxUserAgent(ctx context.Context, log *slog.Logger) string {
	var osStrings = []string{
		"Macintosh; Intel Mac OS X 10_10",
		"Windows NT 10.0",
		"Windows NT 5.1",
		"Windows NT 6.1; WOW64",
		"Windows NT 6.1; Win64; x64",
		"X11; Linux x86_64",
	}
	var ffVersions = []float32{
		58.0,
		57.0,
		56.0,
		52.0,
		48.0,
		40.0,
		35.0,
	}

	versionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(ffVersions))))
	if err != nil {
		log.ErrorContext(ctx, "failed to generate random Firefox version", "error", err)
		return fallbackUserAgent
	}
	version := ffVersions[versionIdx.Int64()]

	osIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(osStrings))))
	if err != nil {
		log.ErrorContext(ctx, "failed to generate random OS", "error", err)
		return fallbackUserAgent
	}
	os := osStrings[osIdx.Int64()]

	return fmt.Sprintf(firefoxTemplate, os, version, version)
}

func genChromeUserAgent(ctx context.Context, log *slog.Logger) string {
	var osStrings = []string{
		"Macintosh; Intel Mac OS X 10_10",
		"Windows NT 10.0",
		"Windows NT 5.1",
		"Windows NT 6.1; WOW64",
		"Windows NT 6.1; Win64; x64",
		"X11; Linux x86_64",
	}
	var chromeVersions = []string{
		"65.0.3325.146",
		"64.0.3282.0",
		"41.0.2228.0",
		"40.0.2214.93",
		"37.0.2062.124",
	}

	versionIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(chromeVersions))))
	if err != nil {
		log.ErrorContext(ctx, "failed to generate random Chrome version", "error", err)
		return fallbackUserAgent
	}
	version := chromeVersions[versionIdx.Int64()]

	osIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(osStrings))))
	if err != nil {
		log.ErrorContext(ctx, "failed to generate random OS", "error", err)
		return fallbackUserAgent
	}
	os := osStrings[osIdx.Int64()]

	return fmt.Sprintf(chromeTemplate, os, version)
}
