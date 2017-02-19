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
	"os"
	"path/filepath"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"github.com/spf13/viper"
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

func filenameNoExt(fname string) string {
	fname = filepath.Base(fname)
	return strings.TrimSuffix(fname, filepath.Ext(fname))
}

func loadConfig() map[string]interface{} {
	config := viper.GetViper()
	config.SetConfigName("config")
	config.AddConfigPath(".")
	config.SetDefault("SourcePagePath", "pages-md")
	config.SetDefault("DestinationPagePath", "pages-html")
	config.SetDefault("GravatarUsername", "")
	config.SetDefault("GravatarEmail", "")
	config.SetDefault("ImagePath", "images")
	config.SetDefault("PageTemplateFile", "template.html")
	config.SetDefault("StyleFile", "style.css")
	err := config.ReadInConfig()
	if err != nil && !strings.Contains(err.Error(), "Not Found") {
		checkError(err)
	}
	return config.AllSettings()
}

func createDirs(conf map[string]interface{}) {
	destPath := conf["destinationpagepath"].(string)
	err := os.Mkdir(destPath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}

	imagePath := conf["imagepath"].(string)
	err = os.Mkdir(imagePath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}
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

func main() {
	conf := loadConfig()
	createDirs(conf)

	mdfiles, err := filepath.Glob(filepath.Join(conf["sourcepagepath"].(string), "*.md"))
	checkError(err)

	npages := len(mdfiles)
	pageList := make([]string, npages)
	plural := func(n int) string {
		if n != 1 {
			return "s"
		}
		return ""
	}
	fmt.Printf("Generating %d page%s\n", npages, plural(npages))
	for idx, fname := range mdfiles {
		fmt.Printf("%d: %s", idx+1, fname)
		pagemd, err := ioutil.ReadFile(fname)
		checkError(err)
		unsafe := blackfriday.MarkdownCommon(pagemd)
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		outName := fmt.Sprintf("%s.html", filenameNoExt(fname))
		outPath := filepath.Join(conf["destinationpagepath"].(string), outName)
		err = ioutil.WriteFile(outPath, makeHTML(safe), 0666)
		fmt.Printf(" → %s\n", outPath)
		pageList[idx] = outPath
	}

	getAvatar(conf)
}
