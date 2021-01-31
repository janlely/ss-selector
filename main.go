package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/go-ping/ping"
)

const (
	//SubURL subscribe url
	SubURL = "https://www.cordcloud.pro/link/BF1R677NnwobgbYP?mu=0"
)

type SSConfig struct {
	ServerAddress string `json:"server"`
	ServerPort    int    `json:"server_port"`
	Method        string `json:"method"`
	Password      string `json:"password"`
	Delay         time.Duration
	LocalAddress  string `json:"local_address"`
	LocalPort     int    `json:"local_port"`
	Timeout       int    `json:"timeout"`
}

func main() {
	fmt.Println("fetching ssr link list data...")
	resp, err := http.Get(SubURL)
	if err != nil {
		fmt.Printf("cannot get ssr links, %v\n", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("error read response for subscrib ssr links, %v\n", err)
	}

	fmt.Println("parsing ssr link list...")
	ssrLinks, err := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(string(body))
	if err != nil {
		fmt.Printf("cannot parse ssr links, body: %v, err: %v\n", string(body), err)
	}

	ssrLinkSlices := strings.Split(string(ssrLinks), "\n")

	var servers []SSConfig
	for i := 0; i < len(ssrLinkSlices); i++ {
		if strings.HasPrefix(ssrLinkSlices[i], "ssr://") {
			ssrLinkB64 := ssrLinkSlices[i][6:]
			ssrLink, _ := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(ssrLinkB64)
			fmt.Println(string(ssrLink))
			servers = append(servers, parse2Json(string(ssrLink)))
		}
	}

	var wg sync.WaitGroup
	count := len(servers)
	bar := pb.StartNew(count)
	fmt.Println("Pinging servers...")
	for i := 0; i < len(servers); i++ {
		go func(idx int) {
			defer wg.Done()
			wg.Add(1)
			pinger, err := ping.NewPinger(servers[idx].ServerAddress)
			if err != nil {
				fmt.Printf("error ping: %v, %v", servers[idx].ServerAddress, err)
			}
			pinger.Count = 3
			stats := pinger.Statistics()
			servers[idx].Delay = stats.AvgRtt
			bar.Increment()
		}(i)
	}
	bar.Finish()
	wg.Wait()
	sort.SliceStable(servers, func(i, j int) bool {
		return servers[i].Delay < servers[j].Delay
	})
	selectedServer, _ := json.Marshal(servers[0])
	fmt.Printf("server selected is: %v\n", string(selectedServer))

}

func parse2Json(input string) SSConfig {
	words := strings.Split(input, ":")
	serverAddress := words[0]
	serverPort, _ := strconv.Atoi(words[1])
	method := words[3]
	password, _ := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(words[5])

	return SSConfig{
		ServerAddress: serverAddress,
		ServerPort:    serverPort,
		Method:        method,
		Password:      string(password),
		Delay:         1 * time.Hour,
		Timeout:       300,
		LocalAddress:  "127.0.0.1",
		LocalPort:     1080,
	}
}
