package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/go-ping/ping"
)

// Duration just packed time.Duration
type Duration struct {
	time.Duration
}

// MarshalJSON json format for Duration
func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(fmt.Sprintf(`"%s"`, d.String())), nil
}

// SSConfig config for ss-locl
type SSConfig struct {
	// ServerAddress server
	ServerAddress string `json:"server"`
	// ServerPort    server_port
	ServerPort int `json:"server_port"`
	// Method        method
	Method string `json:"method"`
	// Password      password
	Password string `json:"password"`
	// Delay         ping avg delay
	Delay Duration `json:"delay"`
	// LocalAddress  local_address
	LocalAddress string `json:"local_address"`
	// LocalPort     local_port
	LocalPort int `json:"local_port"`
	// Timeout       timeout
	Timeout int `json:"timeout"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: SSSelector <subscribe url>")
		os.Exit(-1)
	}
	fmt.Println("fetching ssr link list data...")
	// client := http.Client{
	// 	Timeout: 60 * time.Second,
	// }
	resp, err := http.Get(os.Args[1])
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
			pinger.Timeout = 5 * time.Second
			if err != nil {
				fmt.Printf("error ping: %v, %v", servers[idx].ServerAddress, err)
			}
			pinger.Count = 3
			pinger.Run()
			stats := pinger.Statistics()
			if stats.PacketsRecv == 3 {
				servers[idx].Delay = Duration{stats.AvgRtt}
			}
			bar.Increment()
		}(i)
	}
	wg.Wait()
	bar.Finish()
	sort.SliceStable(servers, func(i, j int) bool {
		return servers[i].Delay.Duration < servers[j].Delay.Duration
	})
	selectedServer, _ := json.MarshalIndent(servers[0], "", "    ")
	configFile, _ := os.Create("./shadowsocks.cfg")
	defer configFile.Close()
	fmt.Printf("server selected is: %v\n", string(selectedServer))
	configFile.Write(selectedServer)

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
		Delay:         Duration{1 * time.Hour},
		Timeout:       300,
		LocalAddress:  "127.0.0.1",
		LocalPort:     1080,
	}
}
