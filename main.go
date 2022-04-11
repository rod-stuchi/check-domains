package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	pb "github.com/cheggaaa/pb/v3"
)

type version struct {
	hash string
	code int
}

type result struct {
	name string
	host string
	elb  bool
	dig  []string
	git  version
}

func main() {
	fmt.Printf("\n")
	tmpl := `{{ white "Checking domains" }} {{counters . | green}} {{ bar . "[" (yellow "=") (yellow ">") "." "]" }} {{percent . | green}}`

	resultChan := make(chan result, 10)

	client := http.Client{
		Timeout: 3 * time.Second,
	}

	count := 0

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		host := scanner.Text()
		name := strings.Split(host, ".")[0]
		go checkDomain(host, name, resultChan, &client)
		count++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	bar := pb.ProgressBarTemplate(tmpl).Start(count)
	bar.SetWidth(90)

	var results []result
	for i := 0; i < count; i++ {
		resChan := <-resultChan
		results = append(results, resChan)
		bar.Increment()
	}

	bar.Finish()

	fmt.Printf("\n\n")

	sort.Slice(results, func(i, j int) bool {
		return results[i].name < results[j].name
	})

	var totalOk int = 0
	var totalNok int = 0

	for _, r := range results {
		fmt.Printf("\x1B[1;97m%s\x1B[0m > https://%s\n", r.name, r.host)
		var dnsok string = "NOK"
		if r.elb {
			totalOk++
			dnsok = "OK"
			fmt.Printf("   DNS: [\x1B[1;92m%s\x1B[0m] \x1B[90m%s\x1B[0m\n", dnsok, strings.Join(r.dig, ", "))
		} else {
			totalNok++
			fmt.Printf("   DNS: [\x1B[1;91m%s\x1B[0m] %s\n", dnsok, strings.Join(r.dig, ", "))
		}

		gitok := "NOK"
		if r.git.code == 200 {
			gitok = "OK"
			fmt.Printf("   git: [\x1B[1;92m%s\x1B[0m] \x1B[90m%s code: %d\x1B[0m\n", gitok, r.git.hash, r.git.code)
		} else {
			fmt.Printf("   git: [\x1B[1;91m%s\x1B[0m] %s code: %d\n", gitok, r.git.hash, r.git.code)
		}
		fmt.Println("")
	}

	fmt.Printf("DNS OK: %d\n", totalOk)
	fmt.Printf("DNS NOK: %d\n", totalNok)

	close(resultChan)
}

func checkDomain(host string, name string, c chan result, client *http.Client) {
	cmd := exec.Command("dig", "A", "+short", host)
	stdout, err := cmd.Output()
	if err != nil {
		log.Println(err)
	}
	out := string(stdout)
	outLines := deleteEmpty(strings.Split(out, "\n"))

	res := result{
		name: name,
		host: host,
		dig:  outLines,
	}

	resp, err := client.Get("https://" + host + "/git")

	if err != nil {
		res.git = version{
			hash: "--",
		}
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		res.git = version{
			code: resp.StatusCode,
		}

		if resp.StatusCode == 200 {
			hash := strings.Replace(string(body), "\n", "", 1)
			if strings.Contains(hash, "<head>") {
				res.git.hash = hash[0:40]
			} else {
				res.git.hash = hash
            }
		}
	}

	for _, host := range outLines {
		match, _ := regexp.MatchString(".*elb.amazonaws.com", host)
		if match {
			res.elb = true
			break
		}
	}

	c <- res
}

func deleteEmpty(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
