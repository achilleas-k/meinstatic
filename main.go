package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
)

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func makeBody(body []byte) []byte {
	return []byte(fmt.Sprintf("<body>%s</body>", body))
}

func readTemplate() string {
	thtml, err := ioutil.ReadFile("template.html")
	checkError(err)
	return string(thtml)
}

func makeHTML(content []byte) []byte {
	thtml := readTemplate()
	t, err := template.New("webpage").Parse(thtml)
	checkError(err)
	rendered := new(bytes.Buffer)
	data := struct {
		Body template.HTML
	}{
		Body: template.HTML(string(content)),
	}
	err = t.Execute(rendered, data)
	return rendered.Bytes()
}

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

func gravatar(username string) {
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

	b, err = ioutil.ReadAll(res.Body)
	checkError(err)

	fmt.Println("Saving to images/avatar.jpg")
	err = ioutil.WriteFile("images/avatar.jpg", b, 0660)
	checkError(err)
}

func filenameNoExt(fname string) string {
	fname = filepath.Base(fname)
	return strings.TrimSuffix(fname, filepath.Ext(fname))
}

func main() {
	mdfiles, err := filepath.Glob("pages/*.md")
	checkError(err)

	fmt.Println("Pages")
	for idx, fname := range mdfiles {
		fmt.Printf("%d: %s\n", idx, fname)
		pagemd, err := ioutil.ReadFile(fname)
		checkError(err)
		unsafe := blackfriday.MarkdownCommon(pagemd)
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		outName := fmt.Sprintf("%s.html", filenameNoExt(fname))
		err = ioutil.WriteFile(outName, makeHTML(safe), 0660)
	}
}
