package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kurrik/oauth1a"
	"github.com/kurrik/twittergo"
	"github.com/manifoldco/promptui"
)

type Config struct {
	APIKey            string `json:"APIKey"`
	APISecretKey      string `json:"APISecretKey"`
	AccessToken       string `json:"AccessToken"`
	AccessTokenSecret string `json:"AccessTokenSecret"`
	Spare []string `json:"Spare,omitempty"`
}

func LoadConfigFrom(ConfigFile string) (client *twittergo.Client, config *Config, err error) {
	credentials, err := ioutil.ReadFile(ConfigFile)
	if err != nil {
		return
	}

	if err = json.Unmarshal(credentials, &config); err != nil {
		return
	}

	UserConfig := oauth1a.NewAuthorizedConfig(config.AccessToken, config.AccessTokenSecret)
	ClientConfig := &oauth1a.ClientConfig{
		ConsumerKey:    config.APIKey,
		ConsumerSecret: config.APISecretKey,
	}
	client = twittergo.NewClient(ClientConfig, UserConfig)
	return
}

type Args struct {
	Days       int
	ConfigFile string
	Help       bool
	Test       bool
}

func parseArgs() *Args {
	a := &Args{}
	flag.StringVar(&a.ConfigFile, "config", "", "JSON Config File")
	flag.IntVar(&a.Days, "days", 0, "Tweets older than this value will be deleted")
	flag.BoolVar(&a.Help, "help", false, "Prints this message")
	flag.BoolVar(&a.Test, "test", false, "Do not delete anything")
	flag.Parse()
	return a
}

// VerifyCredentials from the given client and return a User
// TODO Rate limit
func VerifyCredentials(client *twittergo.Client) (user *twittergo.User, err error) {

	var (
		req  *http.Request
		resp *twittergo.APIResponse
	)

	req, err = http.NewRequest("GET", "/1.1/account/verify_credentials.json", nil)
	resp, err = client.SendRequest(req)
	if err != nil {
		return
	}

	user = &twittergo.User{}
	err = resp.Parse(user)
	return
}

func main() {
	var (
		err     error
		client  *twittergo.Client
		req     *http.Request
		resp    *twittergo.APIResponse
		args    *Args
		max_id  uint64
		query   url.Values
		results *twittergo.Timeline
		config	*Config
	)

	args = parseArgs()

	if client, config, err = LoadConfigFrom(args.ConfigFile); err != nil {
		Usage()
		log.Fatalf("Could not parse config file: %v\n", err)
	}

	if args.Days == 0 {
		fmt.Println("This will delete all of your tweets! Are you sure?")
		if yesNo() != true {
			Usage()
			os.Exit(0)
		}
	}

	user, err := VerifyCredentials(client)
	if err != nil {
		log.Println(err)
	}

	const (
		count   int = 200
		urltmpl     = "/1.1/statuses/user_timeline.json?%v"
		minwait     = time.Duration(10) * time.Second
	)

	query = url.Values{}
	query.Set("count", fmt.Sprintf("%v", count))
	query.Set("screen_name", user.ScreenName())
	total := 0

	for {
		if max_id != 0 {
			query.Set("max_id", fmt.Sprintf("%v", max_id))
		}
		endpoint := fmt.Sprintf(urltmpl, query.Encode())
		if req, err = http.NewRequest("GET", endpoint, nil); err != nil {
			log.Fatalf("Could not parse request: %v\n", err)
		}
		if resp, err = client.SendRequest(req); err != nil {
			log.Fatalf("Could not send request: %v\n", err)
		}
		results = &twittergo.Timeline{}
		if err = resp.Parse(results); err != nil {
			if rle, ok := err.(twittergo.RateLimitError); ok {
				dur := rle.Reset.Sub(time.Now()) + time.Second
				if dur < minwait {
					// Don't wait less than minwait.
					dur = minwait
				}
				msg := "Rate limited. Reset at %v. Waiting for %v\n"
				fmt.Printf(msg, rle.Reset, dur)
				time.Sleep(dur)
				continue // Retry request.
			} else {
				fmt.Printf("Problem parsing response: %v\n", err)
			}
		}
		batch := len(*results)
		if batch == 0 {
			break
		}
		for _, tweet := range *results {
			if contains(config.Spare, tweet.IdStr()) {
				fmt.Printf("Skipping: %v\n", tweet.IdStr())
				max_id = tweet.Id() - 1
				total++
				continue;
			}

			var days int = int(time.Since(tweet.CreatedAt()).Hours() / 24)

			if days >= args.Days {
				fmt.Printf("Tweet: %v\tCreated at: %v\tDays Since Creation:%v\n", tweet.IdStr(), tweet.CreatedAt(), days)
				if !args.Test {
					endpoint := "/1.1/statuses/destroy/" + strconv.FormatUint(tweet.Id(), 10) + ".json"
					data := url.Values{}
					data.Set("id", strconv.FormatUint(tweet.Id(), 10))
					body := strings.NewReader(data.Encode())
					req, err = http.NewRequest("POST", endpoint, body)
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					if err != nil {
						log.Fatalln(err)
					}
					resp, err = client.SendRequest(req)
					if err != nil {
						log.Fatalf("Could not send request: %v\n", err)
					}
					fmt.Printf("Deleted: %v\n", tweet.IdStr())
				}
			}
			max_id = tweet.Id() - 1
			total++
		}
	}
}

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func yesNo() bool {
	prompt := promptui.Select{
		Label: "Select[Yes/No]",
		Items: []string{"Yes", "No"},
	}
	_, result, err := prompt.Run()
	if err != nil {
		log.Fatalf("Prompt failed %v\n", err)
	}
	return result == "Yes"
}
