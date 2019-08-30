// Licensed under MIT license, see LICENSE file.

package main

import (
	"context"
	"flag"
	"os"
	"path"

	//"path/filepath"
	"net/http"
	"regexp"
	"strconv"

	"bytes"
	"strings"
	"time"

	"os/exec"

	"fmt"
	"html"
)

func server(verbosity int, httpServerAddr *string) {
	var addr string
	if httpServerAddr == nil {
		addr = ":9999"
	} else {
		addr = *httpServerAddr
	}
	if verbosity >= 1 {
		fmt.Println("Starting http server at ", addr)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		_ = html.EscapeString(r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - server x.x.x.x is down!"))
	})

	fmt.Fprintln(os.Stderr, http.ListenAndServe(addr, nil))

}

func main() {

	//os.Exit(0)
	// wordPtr := flag.String("word", "foo", "a string")
	// numbPtr := flag.Int("numb", 42, "an int")
	// boolPtr := flag.Bool("fork", false, "a bool")

	repeatPtr := flag.Bool("r", false, "run forever")

	httpServer := flag.Bool("http", false, "enable http server, implies -r")
	httpServerAddr := flag.String("httpaddr", ":8080", "set http server address:port")

	defaultUclientArgs := "-DgX -u user -w pass -n 400 -c -y"
	uclientArgs := flag.String("uclientargs", defaultUclientArgs, "turnutil_uclient args")

	// verbosity
	// 0 totally silent - production recommended
	// 1 show captured stats each run
	// 2 show program starts, and timer
	// 3 super verbose for debugging issues
	verbosity := flag.Int("v", 1, "set verbosity level")

	flag.Parse()

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] hostnames\n", path.Clean(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "after flags at least one hostname must be supplied\n")
	}

	if len(flag.Args()) == 0 {
		fmt.Fprintf(flag.CommandLine.Output(), "fatal error: no hostnames supplied\n\n")
		flag.Usage()
		os.Exit(255)
	}

	if *httpServer {
		if !*repeatPtr {
			foo := true
			repeatPtr = &foo
			fmt.Println("enabling -r for run forever because of -http")
		}

		go server(*verbosity, httpServerAddr)
	}

	for _, host := range flag.Args() {
		go func(hhh string) {
			for {
				tr := performTurnSessionAndPrintStats(*verbosity, hhh, *uclientArgs)
				if *verbosity >= 1 {
					fmt.Printf("%+v\n", tr)
				}
				if !*repeatPtr {
					break
				}
			}
		}(host)
	}

	for {
		time.Sleep(time.Hour)
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

func performTurnSessionAndPrintStats(verbosity int, host string, uclientArgs string) TurnServerTestRun {

	tr := TurnServerTestRun{}
	tr.hostname = host

	uclientArgs = uclientArgs + " " + host

	argsarr := strings.Split(uclientArgs, " ")

	// if turnutils_uclient has an issue we timeout and kill it
	// in an effort toward robustness
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// docs: The provided context is used to kill the process (by calling os.Process.Kill)
	// if the context becomes done before the command completes on its own.
	// cam: the go runtime will kill the process when timeout occurs. verified.
	if verbosity >= 1 {
		fmt.Println("exec:", turnutils_uclient, strings.Join(argsarr, " "))
	}
	cmd := exec.CommandContext(ctx, turnutils_uclient, argsarr...)

	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			if verbosity >= 1 {
				fmt.Print(".")
			}
		}
	}()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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

	errBytes := stderr.Bytes()
	outBytes := stdout.Bytes()

	errStr := string(errBytes)
	outStr := string(outBytes)
	ticker.Stop()

	if verbosity >= 2 {
		fmt.Println(turnutils_uclient, "stdout:")
		fmt.Println(outStr)
		fmt.Println()
		fmt.Println(turnutils_uclient, "stderr:")
		fmt.Println(errStr)
		fmt.Println()
	}

	if len(errStr) > 0 {
		fmt.Fprintln(os.Stderr, turnutils_uclient, "error occurred, exiting: ")
		fmt.Fprintln(os.Stderr, errStr)
		fmt.Fprintln(os.Stderr)
		os.Exit(255)
	}

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
	tr.tot_send_msgs, err = strconv.Atoi(string(sub[1]))
	chk(err)
	tr.tot_recv_msgs, err = strconv.Atoi(string(sub[2]))
	chk(err)

	// 5: start_mclient: tot_send_bytes ~ 800, tot_recv_bytes ~ 800
	re = regexp.MustCompile(`(?m)start_mclient: tot_send_bytes ~ (\d*), tot_recv_bytes ~ (\d*)$`)
	sub = re.FindSubmatch(outBytes)
	tr.tot_send_bytes, err = strconv.Atoi(string(sub[1]))
	chk(err)
	tr.tot_recv_bytes, err = strconv.Atoi(string(sub[2]))
	chk(err)

	re = regexp.MustCompile(`total send dropped (\d*)`)
	sub = re.FindSubmatch(outBytes)
	tr.total_send_dropped, err = strconv.Atoi(string(sub[1]))
	chk(err)

	// 5: Average round trip delay 15.625000 ms; min = 1 ms, max = 56 ms
	re = regexp.MustCompile(`Average round trip delay ([+-]?(?:(?:\d+\.?\d*)|(?:\.\d+))) ms; min = (\d*) ms, max = (\d*) ms`)
	sub = re.FindSubmatch(outBytes)
	tr.round_trip_delay_mean, err = strconv.ParseFloat(string(sub[1]), 64)
	chk(err)
	tr.round_trip_delay_min, err = strconv.Atoi(string(sub[2]))
	chk(err)
	tr.round_trip_delay_max, err = strconv.Atoi(string(sub[3]))
	chk(err)

	// 5: Average jitter 12.500000 ms; min = 1 ms, max = 53 ms
	re = regexp.MustCompile(`Average jitter ([+-]?(?:(?:\d+\.?\d*)|(?:\.\d+)|(?:nan))) ms; min = (\d*) ms, max = (\d*) ms`)
	sub = re.FindSubmatch(outBytes)
	if string(sub[1]) == "-nan" || string(sub[1]) == "nan" {
		sub[1] = []byte("0.0")
	}
	tr.jitter_mean, err = strconv.ParseFloat(string(sub[1]), 64)
	chk(err)
	tr.jitter_min, err = strconv.Atoi(string(sub[2]))
	chk(err)
	tr.jitter_max, err = strconv.Atoi(string(sub[3]))
	chk(err)

	return tr
}

type TurnServerTestRun struct {
	hostname      string `db:"hostname"`
	tot_send_msgs int    `db:"tot_send_msgs"`
	tot_recv_msgs int    `db:"tot_recv_msgs"`

	tot_send_bytes int `db:"tot_send_bytes"`

	tot_recv_bytes     int `db:"tot_recv_bytes"`
	total_send_dropped int `db:"total_send_dropped"`

	round_trip_delay_mean float64 `db:"round_trip_delay_mean"`
	round_trip_delay_min  int     `db:"round_trip_delay_min"`
	round_trip_delay_max  int     `db:"round_trip_delay_max"`

	jitter_mean float64 `db:"jitter_mean"`
	jitter_min  int     `db:"jitter_min"`
	jitter_max  int     `db:"jitter_max"`
	runtime_seconds int
}
