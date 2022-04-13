package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	pb "github.com/cheggaaa/pb/v3"
)

type Version struct {
	hash  string
	code  int
	match bool
}

type Result struct {
	name  string
	host  string
	match bool
	dig   []string
	git   Version
}

type Flags struct {
	gitFlag    *string
	dnsFlag    *string
	hideOkFlag *bool
}

type Totals struct {
	dnsTotalOk  int
	dnsTotalNok int
	gitTotalOk  int
	gitTotalNok int
}

func main() {
	fmt.Printf("\n")
	dnsFlag := flag.String("dns", "", "match DNS entries")
	gitFlag := flag.String("git", "", "match Git hash version")
	hideOkFlag := flag.Bool("hide", false, "hide entries with matching git and dns")
	flag.Parse()

	flags := Flags{
		dnsFlag:    *&dnsFlag,
		gitFlag:    *&gitFlag,
		hideOkFlag: *&hideOkFlag,
	}

	tmpl := `{{ white "Checking domains" }} {{counters . | green}} {{ bar . "[" (yellow "=") (yellow ">") "." "]" }} {{percent . | green}}`

	resultChan := make(chan Result, 10)

	client := http.Client{
		Timeout: 15 * time.Second,
	}

	count := 0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		host := scanner.Text()
		name := strings.Split(host, ".")[0]
		go checkDomain(host, name, resultChan, &client, &flags)
		count++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	bar := pb.ProgressBarTemplate(tmpl).Start(count)
	bar.SetWidth(90)

	var results []Result
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

	var total Totals = Totals{}
	for _, r := range results {
		printResults(r, &flags, &total)
	}

	if *flags.dnsFlag != "" {
		fmt.Printf(" DNS OK: [\x1B[92m%02d\x1B[0m], NOK: [\x1B[91m%02d\x1B[0m]\n", total.dnsTotalOk, total.dnsTotalNok)
	}
	if *flags.gitFlag != "" {
		fmt.Printf(" Git OK: [\x1B[92m%02d\x1B[0m], NOK: [\x1B[91m%02d\x1B[0m]\n", total.gitTotalOk, total.gitTotalNok)
	}

	close(resultChan)
}

func printResults(r Result, flags *Flags, total *Totals) {
	if *flags.hideOkFlag && r.git.match && r.match {
		return
	}
	dnsRg := regexp.MustCompile(fmt.Sprintf("(?P<left>.*)(?P<center>%s)(?P<right>.*)", *flags.dnsFlag))
	gitRg := regexp.MustCompile(fmt.Sprintf("(?P<left>.*)(?P<center>%s)(?P<right>.*)", *flags.gitFlag))

	fmt.Printf("\x1B[1;97m%s\x1B[0m > https://%s\n", r.name, r.host)
	var dnsok string = "NOK"
	if r.match {
		total.dnsTotalOk++
		dnsok = "OK"
		dnsJoined := strings.Join(r.dig, ", ")
		// dnsJoined = dnsRg.ReplaceAllString(dnsJoined, "\x1B[90m${left}\x1B[0m\x1B[48;5;231;90m${center}\x1B[0m\x1B[90m${right}\x1B[0m")
		dnsJoined = dnsRg.ReplaceAllString(dnsJoined, "\x1B[90m${left}\x1B[0m\x1B[1;38;5;229m${center}\x1B[0m\x1B[90m${right}\x1B[0m")
		fmt.Printf("   DNS: [\x1B[1;92m%s\x1B[0m] %s\n", dnsok, dnsJoined)
	} else {
		total.dnsTotalNok++
		fmt.Printf("   DNS: [\x1B[1;91m%s\x1B[0m] %s\n", dnsok, strings.Join(r.dig, ", "))
	}

	gitok := "NOK"
	if r.git.match {
		total.gitTotalOk++
		gitok = "OK"
		gitHash := gitRg.ReplaceAllString(r.git.hash, "\x1B[90m${left}\x1B[0m\x1B[1;38;5;229m${center}\x1B[0m\x1B[90m${right}\x1B[0m")
		fmt.Printf("   git: [\x1B[1;92m%s\x1B[0m] %s \x1B[90mcode: %d\x1B[0m\n", gitok, gitHash, r.git.code)
	} else {
		total.gitTotalNok++
		fmt.Printf("   git: [\x1B[1;91m%s\x1B[0m] %s code: %d\n", gitok, r.git.hash, r.git.code)
	}
	fmt.Println("")
}

func checkDomain(host, name string, c chan Result, client *http.Client, flags *Flags) {
	cmd := exec.Command("dig", "A", "+short", host)
	stdout, err := cmd.Output()
	if err != nil {
		log.Println(err)
	}
	out := string(stdout)
	outLines := deleteEmpty(strings.Split(out, "\n"))

	res := Result{
		name: name,
		host: host,
		dig:  outLines,
	}

	address := "https://" + host

	resp, err := client.Get(address + "/git")

	if err != nil {
		res.git = Version{
			hash: "--",
		}
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		res.git = Version{
			code: resp.StatusCode,
		}

		if resp.StatusCode == 200 {
			hash := strings.Replace(string(body), "\n", "", 1)
			if strings.Contains(hash, "<head>") {
				hash = getPWAGitHash(address)
			}
			match, _ := regexp.MatchString(*flags.gitFlag, hash)
			if match {
				res.git.match = true
			}
			res.git.hash = hash
		}
	}

	if *flags.dnsFlag == "" {
		res.match = true
	} else {
		for _, host := range outLines {
			match, _ := regexp.MatchString(*flags.dnsFlag, host)
			if match {
				res.match = true
				break
			}
		}
	}

	c <- res
}

func getPWAGitHash(address string) string {
	rg := regexp.MustCompile("HEAD VERSION: [^)]+")
	var gitHash string

	res, err := http.Get(address)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if strings.Contains(src, "/static/js/main") {
			res, err := http.Get(address + src)
			if err != nil {
				log.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
			}

			if b, err := io.ReadAll(res.Body); err == nil {
				hash := rg.FindStringSubmatch(string(b))
				if len(hash) > 0 {
					hashv := strings.Split(hash[0], ",")
					gitHash = hashv[1]
				}
			}
		}
	})

	return gitHash
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
