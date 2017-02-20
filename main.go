package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
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

func makeHTML(data templateData) []byte {
	thtml := readTemplate()
	t, err := template.New("webpage").Parse(thtml)
	checkError(err)
	rendered := new(bytes.Buffer)
	err = t.Execute(rendered, data)
	return rendered.Bytes()
}

func filenameNoExt(fname string) string {
	fname = filepath.Base(fname)
	return strings.TrimSuffix(fname, filepath.Ext(fname))
}

func loadConfig() map[string]interface{} {
	config := viper.GetViper()
	config.SetConfigName("config")
	config.AddConfigPath(".")
	config.SetDefault("SiteName", "")
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

type templateData struct {
	SiteName template.HTML
	Body     []template.HTML
}

func renderPages(conf map[string]interface{}) {
	mdfiles, err := filepath.Glob(filepath.Join(conf["sourcepagepath"].(string), "*.md"))
	checkError(err)

	sitename := conf["sitename"].(string)
	var data templateData

	data.SiteName = template.HTML(sitename)

	npages := len(mdfiles)
	pageList := make([]string, npages)
	data.Body = make([]template.HTML, npages)
	plural := func(n int) string {
		if n != 1 {
			return "s"
		}
		return ""
	}

	fmt.Printf("Rendering %d page%s\n", npages, plural(npages))
	for idx, fname := range mdfiles {
		fmt.Printf("%d: %s", idx+1, fname)
		pagemd, err := ioutil.ReadFile(fname)
		checkError(err)

		unsafe := blackfriday.MarkdownCommon(pagemd)
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		data.Body[npages-idx-1] = template.HTML(string(safe))

		outName := fmt.Sprintf("%s.html", filenameNoExt(fname))
		outPath := filepath.Join(conf["destinationpagepath"].(string), outName)
		// err = ioutil.WriteFile(outPath, makeHTML(data), 0666)
		checkError(err)

		fmt.Printf(" → %s\n", outPath)
		pageList[idx] = outPath
	}
	outPath := filepath.Join(conf["destinationpagepath"].(string), "posts.html")
	fmt.Printf("Saving posts: %s\n", outPath)
	err = ioutil.WriteFile(outPath, makeHTML(data), 0666)
	checkError(err)
	fmt.Print("Rendering complete.\n\n")
}

func main() {
	conf := loadConfig()
	createDirs(conf)
	renderPages(conf)
	getAvatar(conf)
}
