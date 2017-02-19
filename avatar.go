package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type gravatarJSON struct {
	Entry []struct {
		ID                string `json:"id"`
		Hash              string `json:"hash"`
		RequestHash       string `json:"requestHash"`
		ProfileURL        string `json:"profileUrl"`
		PreferredUsername string `json:"preferredUsername"`
		ThumbnailURL      string `json:"thumbnailUrl"`
		Photos            []struct {
			Value string `json:"value"`
			Type  string `json:"type"`
		} `json:"photos"`
		Name struct {
			GivenName string `json:"givenName"`
		} `json:"name"`
		DisplayName string        `json:"displayName"`
		Urls        []interface{} `json:"urls"`
	} `json:"entry"`
}

func gravatar(username string) []byte {
	fmt.Println("Fetching Gravatar")
	gravURL := fmt.Sprintf("https://en.gravatar.com/%s.json", url.PathEscape(username))
	res, err := http.Get(gravURL)
	checkError(err)
	b, err := ioutil.ReadAll(res.Body)
	checkError(err)

	var grav gravatarJSON
	err = json.Unmarshal(b, &grav)
	checkError(err)

	imgURL := grav.Entry[0].Photos[0].Value
	imgsize := 160
	imgURL = fmt.Sprintf("%s?s=%d", imgURL, imgsize)
	fmt.Printf("Downloading image [%s]\n", imgURL)
	res, err = http.Get(imgURL)
	checkError(err)

	imgbytes, err := ioutil.ReadAll(res.Body)
	checkError(err)

	return imgbytes
}

func getAvatar(conf map[string]interface{}) {
	imgpath := filepath.Join(conf["imagepath"].(string), "avatar.jpg")

	if _, err := os.Stat(imgpath); os.IsNotExist(err) {
		gravatarUser := conf["gravatarusername"].(string)
		if gravatarUser != "" {
			avatar := gravatar(gravatarUser)
			fmt.Printf("Saving to %s\n", imgpath)
			err = ioutil.WriteFile(imgpath, avatar, 0666)
			checkError(err)
		}
	}
	fmt.Printf("Found profile picture [%s]. Will not download.\n", imgpath)
}