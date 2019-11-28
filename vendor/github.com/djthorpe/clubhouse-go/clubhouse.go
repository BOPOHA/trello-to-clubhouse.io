package clubhouse

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
)

const apiURL string = "https://api.clubhouse.io/api/v2/"

// Clubhouse is a struct containing the token, and the http.Client used for sending the data to the clubhouse API.
type Clubhouse struct {
	Token  string
	Client *http.Client
	Debug  bool
}

// transport is an http.RoundTripper that keeps track of the in-flight
// request and implements hooks to report HTTP tracing events.
type transport struct {
	current *http.Request
}

// New creates a new instance of the Clubhouse object that is used to send data to ClubHouse
func New(token string) *Clubhouse {
	return &Clubhouse{
		Token:  token,
		Client: &http.Client{},
	}
}

// Set the debug value
func (ch *Clubhouse) SetDebug(debug bool) *Clubhouse {
	ch.Debug = debug
	return ch
}

func (ch *Clubhouse) getURL(resource string) string {
	return fmt.Sprintf("%s%s?token=%s", apiURL, resource, ch.Token)
}

func (ch *Clubhouse) getDownloadUrl(resource string) (string, error) {
	if url, err := url.Parse(resource); err != nil {
		return "", err
	} else if url.Host == "api.clubhouse.io" {
		return fmt.Sprintf("%s?token=%s", resource, ch.Token), nil
	} else {
		return resource, nil
	}
}

func (ch *Clubhouse) getResource(resource string) ([]byte, error) {
	req, err := http.NewRequest("GET", ch.getURL(resource), nil)
	if err != nil {
		return []byte{}, err
	}
	if ch.Debug {
		fmt.Printf("=>%v\n", req.URL.String())
	}
	resp, err := ch.Client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	if ch.Debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, err
		}
		fmt.Printf("%q\n", dump)
	}
	if ch.Debug {
		fmt.Printf("<=%v\n", resp.Status)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return []byte{}, fmt.Errorf("API Returned HTTP Status Code of %d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func (ch *Clubhouse) updateResource(resource string, jsonStr []byte) ([]byte, error) {
	req, err := http.NewRequest("PUT", ch.getURL(resource), bytes.NewBuffer(jsonStr))
	if err != nil {
		return []byte{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := ch.Client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return []byte{}, fmt.Errorf("API Returned HTTP Status Code of %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

func (ch *Clubhouse) deleteResource(resource string) error {
	req, err := http.NewRequest("DELETE", ch.getURL(resource), nil)
	if err != nil {
		return err
	}

	resp, err := ch.Client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("API Returned HTTP Status Code of %d", resp.StatusCode)
	}

	return nil
}

func (ch *Clubhouse) listResources(resource string) ([]byte, error) {
	req, err := http.NewRequest("GET", ch.getURL(resource), nil)
	if err != nil {
		return []byte{}, err
	}
	if ch.Debug {
		fmt.Printf("=>%v\n", req.URL.String())
	}
	resp, err := ch.Client.Do(req)
	if err != nil {
		return []byte{}, err
	}
	if ch.Debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return nil, err
		}
		fmt.Printf("%q\n", dump)
	}
	defer resp.Body.Close()
	if ch.Debug {
		fmt.Printf("<=%v\n", resp.Status)
	}
	if resp.StatusCode != 200 {
		return []byte{}, fmt.Errorf("API Returned HTTP Status Code of %d", resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func (ch *Clubhouse) createObject(resource string, jsonStr []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", ch.getURL(resource), bytes.NewBuffer(jsonStr))
	if err != nil {
		return []byte{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := ch.Client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return []byte{}, fmt.Errorf("API Returned HTTP Status Code of %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}
