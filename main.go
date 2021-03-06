// Package main is the entry point for the htttpsyet binary.
// Here is where you can find argument parsing, usage information and the actual execution.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"

	"qvl.io/httpsyet/httpsyet"
	"qvl.io/httpsyet/internal/slack"
	"qvl.io/httpsyet/slackhook"
)

// Can be set in build step using -ldflags
var version string

const (
	// Printed for -help, -h or with wrong number of arguments
	usage = `Find links you can update to HTTPS

Usage: %s [flags] url...

  url  one or more URLs you like to be crawled

Sites are crawled recursively. Each http:// link is checked
to see if it can be replaced with https://. If a link can be replaced,
it is written to stdout, prefixed with the site name it has been found on.
For example:

	httpsyet https://mysite.com

Might output:
	https://mysite.com http://google.com
	https://mysite.com http://facebook.com
	https://mysite.com/contact http://facebook.com
	...

Errors are reported on stderr.

'httpsyet -parallel 5 -delay 1s' means that you will have max 5 requests per second.

Flags:
`
	more = "\nFor more visit https://qvl.io/httpsyet."
)

// Get command line arguments and start crawling
func main() {
	// Flags
	slackURL := flag.String("slack", "", "Slack incoming webhook. If set, results are also posted to Slack. See https://api.slack.com/incoming-webhooks.")
	depth := flag.Int("depth", 0, "Set to >=1 to specify how many layers of pages to crawl.")
	parallel := flag.Int("parallel", 10, "Value needs to be >= 1. Specify how many parallel requests are made per domain.")
	delay := flag.Duration("delay", time.Second, "Delay between requests.")
	versionFlag := flag.Bool("version", false, "Print binary version.")
	verbose := flag.Bool("verbose", false, "Output status updates to standard error.")

	// Parse args
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, more)
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("httpsyet %s %s %s\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	sites := flag.Args()
	if len(sites) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var output io.Writer = os.Stdout
	var errWriter io.Writer = os.Stderr
	var slackBuf, slackErrBuf bytes.Buffer
	if *slackURL != "" {
		output = io.MultiWriter(output, &slackBuf)
		errWriter = io.MultiWriter(errWriter, &slackErrBuf)
	}
	errs := log.New(errWriter, "", 0)

	err := httpsyet.Crawler{
		Sites:    sites,
		Out:      output,
		Log:      errs,
		Depth:    *depth,
		Parallel: *parallel,
		Delay:    *delay,
		Verbose:  *verbose,
	}.Run()

	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to crawl: %v", err)
		os.Exit(1)
	}

	if *slackURL == "" {
		return
	}

	msg := slack.Format(slackBuf.String(), slackErrBuf.String())
	if err := slackhook.Post(*slackURL, msg); err != nil {
		errs.Printf("failed posting to Slack: %v", err)
		os.Exit(1)
	}
}
