package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const Version = "v0.3"

func getNewRelicToken() string {
	var cmd *exec.Cmd
	f := "/etc/newrelic/nrsysmond.cfg"
	if _, err := os.Stat(f); os.IsNotExist(err) {
		log.Fatalf("File does not exist: %s", f)
	}
	cmd = exec.Command("bash", "-c", fmt.Sprintf("grep license_key= %s | cut -d '=' -f2", f))
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Missing a valid New Relic license key: %s", err)
	}
	// log.Print(string(output)) need to notice errors - stdout and stderr are mixed
	return strings.TrimSpace(string(output))
}

func getFullestDisk() int {
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "df --output=source,pcent | grep '^/dev' | sort -rk2 | tail -1 | grep -oE '[0-9]+%' | tr -d '%'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		log.Print(err)
	}
	return v
}

func getCPU() int {
	var cmd *exec.Cmd
	// the first line of top's output should be disregarded
	cmd = exec.Command("bash", "-c", "top -bn 4 -d 0.5 | grep '^%Cpu' | tail -3 | awk '{sum1 += $2} {sum2 += $4} {sum3 += $6} END {print sum1/3 + sum2/3 + sum3/3}'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return int(v)
}

func getMem() int {
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "free | grep Mem | awk '{print $3/$2 * 100}'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return int(v)
}

func getSwap() int {
	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", "free | grep Swap | awk '{print $3/$2 * 100}'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	return int(v)
}

func main() {
	log.SetPrefix(fmt.Sprintf("newrelic-monitor %s ", Version))
	token := getNewRelicToken()

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Can't determine hostname")
	}

	// Poll the system every 60 seconds for updated metrics
	for {
		var jsonStr = []byte(fmt.Sprintf(`
      {
        "agent": {
          "host": "%s",
          "version": "%s"
        },
        "components": [
          {
            "name": "%s",
            "guid": "au.com.lexer.plugin.Servers",
            "duration": 60,
            "metrics": {
              "Component/CPU[percent]": %d,
              "Component/Disk[percent]": %d,
              "Component/Memory[percent]": %d,
              "Component/Swap[percent]": %d
            }
          }
        ]
      }
		`, hostname, Version, hostname, getCPU(), getFullestDisk(), getMem(), getSwap()))

		req, err := http.NewRequest("POST", "https://platform-api.newrelic.com/platform/v1/metrics", bytes.NewBuffer(jsonStr))
		if err != nil {
			log.Printf("Error: %s", err)
		}

		req.Header.Set("X-License-Key", token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error: %s", err)
		} else {
			defer resp.Body.Close()
			l := string(jsonStr)
			l = strings.Replace(l, "\n", " ", -1)
			l = strings.Replace(l, " ", "", -1)
			l = strings.TrimSpace(l)
			log.Println("Sent: " + l + " and received: " + resp.Status)
		}
		time.Sleep(60 * time.Second)
	}
}
