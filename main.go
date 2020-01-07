package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

var botName = "social-checker"
var botVersion = "0.0.1"
var userAgentName = fmt.Sprintf("%s-%s", botName, botVersion)

type availabilityChecker func(*http.Response) (bool, error)

func isStatusCode(code int) availabilityChecker {
	return func(resp *http.Response) (bool, error) {
		return resp.StatusCode == code, nil
	}
}

func bodyEquals(body string) availabilityChecker {
	return func(resp *http.Response) (bool, error) {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return false, errors.Wrap(err, "reading response body failed")
		}
		return string(bodyBytes) == body, nil
	}
}

type website struct {
	name            string
	url             string
	isAvailableFunc availabilityChecker
}

var client = &http.Client{}

func (w website) isAvailable(username string) (bool, error) {
	url := fmt.Sprintf(w.url, username)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, errors.Wrap(err, "creating request failed")
	}
	req.Header.Add("User-agent", userAgentName)
	resp, err := client.Do(req)
	if err != nil {
		return false, errors.Wrap(err, "get request failed")
	}
	defer resp.Body.Close()
	isAvailable, err := w.isAvailableFunc(resp)
	if err != nil {
		return false, errors.Wrap(err, "isAvailableFunc failed")
	}
	return isAvailable, nil
}

type availableWebsiteResult struct {
	website     website
	isAvailable bool
	err         error
}

func availableWebsites(username string, websites []website) ([]website, []website, error) {
	available := []website{}
	unavailable := []website{}
	availableResults := make(chan availableWebsiteResult)
	for _, w := range websites {
		go func(website website) {
			isAvailable, err := website.isAvailable(username)
			availableResults <- availableWebsiteResult{
				website,
				isAvailable,
				err,
			}
		}(w)
	}
	for i := 0; i < len(websites); i++ {
		r := <-availableResults
		if r.err != nil {
			return nil, nil, errors.Wrap(r.err, "checking if website available failed")
		}
		if r.isAvailable {
			available = append(available, r.website)
		} else {
			unavailable = append(unavailable, r.website)
		}
	}
	return available, unavailable, nil
}

func main() {
	websites := []website{
		{
			"Twitch",
			"https://passport.twitch.tv/usernames/%s",
			isStatusCode(204),
		},
		{
			"Twitter",
			"https://twitter.com/%s",
			isStatusCode(404),
		},
		{
			"Instagram",
			"https://www.instagram.com/%s",
			isStatusCode(404),
		},
		{
			"Reddit",
			"https://www.reddit.com/api/username_available.json?user=%s",
			bodyEquals("true"),
		},
		{
			"Subreddit",
			"https://www.reddit.com/r/%s",
			isStatusCode(404),
		},
	}

	username := os.Args[1]

	_, unavailable, err := availableWebsites(username, websites)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(unavailable) == 0 {
		fmt.Println("Available!")
		return
	}
	fmt.Println("Unavailable on:")
	for _, w := range unavailable {
		fmt.Println("- " + w.name)
	}
}
