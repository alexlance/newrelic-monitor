package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const Version = "v0.5"

type EC2InstanceType struct {
	Name                 string
	MaximumCredits       float64
	CreditsEarnedPerHour float64
}

func getInstanceId() (string, error) {
	rep, err := http.Get("http://instance-data/latest/meta-data/instance-id")
	if err != nil {
		return "", err
	}

	defer rep.Body.Close()
	body, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func getInstanceDetails() (EC2InstanceType, error) {
	var instance EC2InstanceType

	instanceDetails := map[string]EC2InstanceType{
		"t2.nano": EC2InstanceType{
			MaximumCredits:       72,
			CreditsEarnedPerHour: 3,
		},
		"t2.micro": EC2InstanceType{
			MaximumCredits:       144,
			CreditsEarnedPerHour: 6,
		},
		"t2.small": EC2InstanceType{
			MaximumCredits:       288,
			CreditsEarnedPerHour: 12,
		},
		"t2.medium": EC2InstanceType{
			MaximumCredits:       576,
			CreditsEarnedPerHour: 24,
		},
		"t2.large": EC2InstanceType{
			MaximumCredits:       864,
			CreditsEarnedPerHour: 36,
		},
		"t2.xlarge": EC2InstanceType{
			MaximumCredits:       1296,
			CreditsEarnedPerHour: 54,
		},
		"t2.2xlarge": EC2InstanceType{
			MaximumCredits:       1958.4,
			CreditsEarnedPerHour: 81.6,
		},
		"t3.nano": EC2InstanceType{
			MaximumCredits:       144,
			CreditsEarnedPerHour: 6,
		},
		"t3.micro": EC2InstanceType{
			MaximumCredits:       288,
			CreditsEarnedPerHour: 12,
		},
		"t3.small": EC2InstanceType{
			MaximumCredits:       576,
			CreditsEarnedPerHour: 24,
		},
		"t3.medium": EC2InstanceType{
			MaximumCredits:       576,
			CreditsEarnedPerHour: 24,
		},
		"t3.large": EC2InstanceType{
			MaximumCredits:       864,
			CreditsEarnedPerHour: 36,
		},
		"t3.xlarge": EC2InstanceType{
			MaximumCredits:       2304,
			CreditsEarnedPerHour: 96,
		},
		"t3.2xlarge": EC2InstanceType{
			MaximumCredits:       4608,
			CreditsEarnedPerHour: 192,
		},
	}

	rep, err := http.Get("http://instance-data/latest/meta-data/instance-type")
	if err != nil {
		return instance, err
	}

	defer rep.Body.Close()
	body, err := ioutil.ReadAll(rep.Body)
	if err != nil {
		return instance, err
	}

	return instanceDetails[string(body)], nil
}

func getNewRelicToken() string {
	t := strings.TrimSpace(os.Getenv("NEWRELIC_TOKEN"))
	if t == "" {
		log.Fatalf("Error: NEWRELIC_TOKEN is not set")
	}
	return t
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

//getCredit returns remaining CPU credits in percentage
func getCredit() int {
	instanceId, err := getInstanceId()

	if err != nil {
		return 0
	}

	commandStr := "aws cloudwatch get-metric-statistics --namespace AWS/EC2 --metric-name CPUCreditBalance --region ap-southeast-2 --dimensions Name=InstanceId,Value=" + instanceId + " --start-time $(date -d '5 minute ago' +%s) --end-time=$(date +%s) --period 3600 --statistics Minimum --unit Count | jq '.Datapoints[0].Minimum'"

	var cmd *exec.Cmd
	cmd = exec.Command("bash", "-c", commandStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}

	v, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0
	}

	instance, err := getInstanceDetails()
	if err != nil {
		return 0
	}

	return int((v / instance.MaximumCredits) * 100)
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
              "Component/Swap[percent]": %d,
              "Component/Credit[percent]": %d
            }
          }
        ]
      }
		`, hostname, Version, hostname, getCPU(), getFullestDisk(), getMem(), getSwap(), getCredit()))

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
