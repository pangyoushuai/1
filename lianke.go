package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/takama/daemon"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (

	// name of the service
	name        = "lianke"
	description = "lianke market trade data"

	// port which daemon should be listen
	port = ":12345"
)

var stdlog, errlog *log.Logger

type Platform struct {
	Name   string `json:"name"`
	Deal   string `json:"deal"`
	Price  string `json:"price"`
	Volume string `json:"volume"`
	Change string `json:"change"`
}

type PlatformList struct {
	Platforms []Platform `json:"platforms"`
}

func StringToLines(s string) []string {
	var lines []string

	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}

	return lines
}

func processTr(tr *goquery.Selection) Platform {
	var p Platform
	tr.Find("td").Each(func(indexOfTd int, td *goquery.Selection) {
		lines := StringToLines(td.Text())
		var line string
		for i := 0; i < len(lines); i++ {
			line = strings.TrimSpace(lines[i])
			switch indexOfTd {
			case 0:
				p.Name = line
			case 1:
				p.Deal = line
			case 2:
				p.Price = line
			case 3:
				p.Volume = line
			case 4:
				p.Change = line
			default:
			}
		}
	})

	return p
}

func htmlTableToRst(inputTable []byte) PlatformList {
	var list PlatformList
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(inputTable))
	if err != nil {
		return list
	}

	doc.Find("table").Each(func(_ int, table *goquery.Selection) {
		table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
			p := processTr(tr)
			if p.Name != "" {
				list.Platforms = append(list.Platforms, p)
			}
		})
	})

	return list
}

func httpDo(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{}

	req, err := http.NewRequest("POST", "http://wkbcom.com/wkbcom.php", strings.NewReader("name=cjb"))
	if err != nil {
		// handle error
	}

	req.Header.Set("Content-Length", "0")
	req.Header.Set("Cookie", "name=anny")

	resp, err := client.Do(req)

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// handle error
	}

	list := htmlTableToRst(body)

	b, err := json.Marshal(list)
	if err != nil {
		fmt.Println("json err:", err)
	}
	w.Write(b)
}

type liankeHandler struct{}

func (h *liankeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpDo(w, r)
}

// Service has embedded daemon
type Service struct {
	daemon.Daemon
}

// Manage by daemon commands or run the daemon
func (service *Service) Manage() (string, error) {

	usage := "Usage: myservice install | remove | start | stop | status"

	// if received any kind of command, do it
	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "install":
			return service.Install()
		case "remove":
			return service.Remove()
		case "start":
			return service.Start()
		case "stop":
			return service.Stop()
		case "status":
			return service.Status()
		default:
			return usage, nil
		}
	}

	http.Handle("/", &liankeHandler{})
	http.ListenAndServe(port, nil)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, os.Kill, syscall.SIGTERM)

	// loop work cycle with accept connections or interrupt
	// by system signal
	for {
		select {
		case killSignal := <-interrupt:
			stdlog.Println("Got signal:", killSignal)
			if killSignal == os.Interrupt {
				return "Daemon was interruped by system signal", nil
			}
			return "Daemon was killed", nil
		}
	}

	// never happen, but need to complete code
	return usage, nil
}

func init() {
	stdlog = log.New(os.Stdout, "", log.Ldate|log.Ltime)
	errlog = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

func main() {
	srv, err := daemon.New(name, description)
	if err != nil {
		errlog.Println("Error: ", err)
		os.Exit(1)
	}
	service := &Service{srv}
	status, err := service.Manage()
	if err != nil {
		errlog.Println(status, "\nError: ", err)
		os.Exit(1)
	}
	fmt.Println(status)
}
