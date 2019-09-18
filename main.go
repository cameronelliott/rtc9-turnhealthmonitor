// Provided under MIT license, see LICENSE file.

package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path"
	"sync"

	//"path/filepath"
	"net/http"
	"regexp"
	"strconv"

	"bytes"
	"strings"
	"time"

	"os/exec"

	"fmt"
	//"html"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func server(verbose bool, httpServerAddr string) {

	if verbose {
		fmt.Println("Starting http server at ", httpServerAddr)
	}

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	http.Handle("/metrics", promhttp.Handler())
	//log.Fatal(http.ListenAndServe(":8080", nil))

	err := http.ListenAndServe(httpServerAddr, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(255)
	}

}

var (
	uclientArgs = flag.String("a", "", "turnutil_uclient arguments, REQUIRED!")
	verbose     = flag.Bool("v", false, "verbose mode")
	sourcename  = flag.String("sourcename", "", "Prometheus label, ie: seattle or chicago")
)

func main() {

	//if you use -n 3000 with uclient, you may want to adjust this to 3, for example
	//timeoutMinutes := flag.Int("timeout", 1, "how many minutes to wait for uclient before timing out")

	var timeout int = 5
	timeoutMinutes := &timeout

	myname := path.Base(os.Args[0])

	var usagex = `
mini-tutorial:
  if you have a working 'turnutils_uclient' command:

  $ turnutils_uclient -DgX -n 500 -c -y -u user -w pass 192.168.2.1
  
  That command would become:
  
  $ %s -a "-DgX -n 500 -c -y -u user -w pass" 192.168.2.1
`

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] hostnames\n\n", myname)
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), usagex, myname)
	}

	flag.Parse()

	if *uclientArgs == "" {
		fmt.Fprintf(flag.CommandLine.Output(), "error: missing -a\n\n")
		flag.Usage()
		os.Exit(255)
	}

	if len(flag.Args()) == 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "error: missing hostnames\n\n")
		flag.Usage()
		os.Exit(255)
	}

	var wg sync.WaitGroup

	for _, host := range flag.Args() {
		wg.Add(1)
		go func(hhh string) {
			defer wg.Done()
			for {
				tr := performTurnSessionAndPrintStats(*timeoutMinutes, *verbose, hhh, *uclientArgs)
				if *verbose {
					fmt.Printf("\nCaptured results:\n%+v\n", tr)
				}
				updatePrometheus(*sourcename, hhh, tr)

			}
		}(host)
	}

	if *verbose {
		fmt.Println("Main: Waiting for workers to finish")
	}

	wg.Wait()

	if *verbose {
		fmt.Println("Main: Completed")
	}

}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

// func printOutput(outs []byte) {
// 	if len(outs) > 0 {
// 		fmt.Printf("==> Output: %s\n", string(outs))
// 	}
// }

const turnutils_uclient = "turnutils_uclient"

func performTurnSessionAndPrintStats(timeoutMinutes int, verbose bool, host string, uclientArgs string) TurnServerTestRun {

	tr := TurnServerTestRun{}
	tr.hostname = host

	uclientArgs = uclientArgs + " " + host

	argsarr := strings.Split(uclientArgs, " ")

	// if turnutils_uclient has an issue we timeout and kill it
	// in an effort toward robustness
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	// docs: The provided context is used to kill the process (by calling os.Process.Kill)
	// if the context becomes done before the command completes on its own.
	// cam: the go runtime will kill the process when timeout occurs. verified.
	if verbose {
		fmt.Println("exec:", turnutils_uclient, strings.Join(argsarr, " "))
	}
	cmd := exec.CommandContext(ctx, turnutils_uclient, argsarr...)

	var stdout, stderr bytes.Buffer
	//cmd.Stdout = &stdout
	//cmd.Stderr = &stderr

	if !verbose {
		cmd.Stdout = io.MultiWriter(&stdout)
		cmd.Stderr = io.MultiWriter(&stderr)
	} else {
		cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	}

	//cmd.Stdout=ioutil.Discard
	//cmd.Stderr=ioutil.Discard

	start := time.Now()

	err := cmd.Run()
	if err != nil {

		fmt.Fprintln(os.Stderr, "error running: ", turnutils_uclient, err.Error())

		// was ExitError? meaning program started, but had non-zero exit code
		// dereferenc with ok meaning yes, of type exiterror
		if _, ok := err.(*exec.ExitError); ok {
			//waitStatus := exitError.Sys().(syscall.WaitStatus)
			//exitStatus := waitStatus.ExitStatus()
			// redundant with above
			//fmt.Println("exit status code = ", exitStatus) //redundant

		} else { //not errorexit
			chk(err)
		}
	}

	tr.elapsed_seconds = int(time.Since(start).Seconds())

	errBytes := stderr.Bytes()
	tr.stderr_bytes = len(errBytes)

	// we don't do anything more with captured std error than report the length,
	// but that's enough for diligent devops people to investigate.
	// writing captured stderr to stdout or stderr won't necessarily get noticed.

	outBytes := stdout.Bytes()

	// 5: start_mclient: tot_send_msgs=8, tot_recv_msgs=8
	// 5: start_mclient: tot_send_bytes ~ 800, tot_recv_bytes ~ 800
	// 5: Total transmit time is 5
	// 5: Total lost packets 0 (0.000000%), total send dropped 0 (0.000000%)
	// 5: Average round trip delay 15.625000 ms; min = 1 ms, max = 56 ms
	// 5: Average jitter 12.500000 ms; min = 1 ms, max = 53 ms

	//fmt.Printf("%q\n", )
	var re *regexp.Regexp
	var sub [][]byte

	re = regexp.MustCompile(`(?m)start_mclient: tot_send_msgs=(\d*), tot_recv_msgs=(\d*)$`)
	sub = re.FindSubmatch(outBytes)
	if len(sub) == 3 {
		tr.tot_send_msgs, err = strconv.Atoi(string(sub[1]))
		_ = err
		//chk(err)
		tr.tot_recv_msgs, err = strconv.Atoi(string(sub[2]))
		_ = err
		//chk(err)
	}

	// 5: start_mclient: tot_send_bytes ~ 800, tot_recv_bytes ~ 800
	re = regexp.MustCompile(`(?m)start_mclient: tot_send_bytes ~ (\d*), tot_recv_bytes ~ (\d*)$`)
	sub = re.FindSubmatch(outBytes)
	if len(sub) == 3 {
		tr.tot_send_bytes, err = strconv.Atoi(string(sub[1]))
		_ = err
		//chk(err)
		tr.tot_recv_bytes, err = strconv.Atoi(string(sub[2]))
		_ = err
		//chk(err)
	}

	re = regexp.MustCompile(`total send dropped (\d*)`)
	sub = re.FindSubmatch(outBytes)
	if len(sub) == 2 {
		tr.total_send_dropped, err = strconv.Atoi(string(sub[1]))
		_ = err
		//chk(err)
	}

	// 5: Average round trip delay 15.625000 ms; min = 1 ms, max = 56 ms
	re = regexp.MustCompile(`Average round trip delay ([+-]?(?:(?:\d+\.?\d*)|(?:\.\d+))) ms; min = (\d*) ms, max = (\d*) ms`)
	sub = re.FindSubmatch(outBytes)
	if len(sub) == 4 {
		tr.round_trip_delay_mean, err = strconv.ParseFloat(string(sub[1]), 64)
		_ = err
		//chk(err)
		tr.round_trip_delay_min, err = strconv.Atoi(string(sub[2]))
		_ = err
		//chk(err)
		tr.round_trip_delay_max, err = strconv.Atoi(string(sub[3]))
		_ = err
		//chk(err)
	}

	// 5: Average jitter 12.500000 ms; min = 1 ms, max = 53 ms
	re = regexp.MustCompile(`Average jitter ([+-]?(?:(?:\d+\.?\d*)|(?:\.\d+)|(?:nan))) ms; min = (\d*) ms, max = (\d*) ms`)
	sub = re.FindSubmatch(outBytes)
	if len(sub) == 4 {
		if string(sub[1]) == "-nan" || string(sub[1]) == "nan" {
			sub[1] = []byte("0.0")
		}
		tr.jitter_mean, err = strconv.ParseFloat(string(sub[1]), 64)
		_ = err
		//chk(err)
		tr.jitter_min, err = strconv.Atoi(string(sub[2]))
		_ = err
		//chk(err)
		tr.jitter_max, err = strconv.Atoi(string(sub[3]))
		_ = err
		//chk(err)
	}

	return tr
}

type TurnServerTestRun struct {
	hostname      string
	tot_send_msgs int
	tot_recv_msgs int

	tot_send_bytes int

	tot_recv_bytes     int
	total_send_dropped int

	round_trip_delay_mean float64
	round_trip_delay_min  int
	round_trip_delay_max  int

	jitter_mean     float64
	jitter_min      int
	jitter_max      int
	elapsed_seconds int

	stderr_bytes int
}
