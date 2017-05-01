package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"
	"github.com/spf13/viper"
)

type templateData struct {
	SiteName template.HTML
	Body     []template.HTML
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func copyFile(srcName, dstName string) error {
	data, err := ioutil.ReadFile(srcName)
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(dstName, data, 0666)
}

func makeBody(body []byte) []byte {
	return []byte(fmt.Sprintf("<body>%s</body>", body))
}

func readTemplate(templateFile string) string {
	thtml, err := ioutil.ReadFile(templateFile)
	checkError(err)
	return string(thtml)
}

func makeHTML(data templateData, templateFile string) []byte {
	thtml := readTemplate(templateFile)
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
	config.SetDefault("SourcePath", "pages-md")
	config.SetDefault("SourcePostsPath", "pages-md/posts")
	config.SetDefault("DestinationPath", "html")
	config.SetDefault("GravatarUsername", "")
	config.SetDefault("GravatarEmail", "")
	config.SetDefault("PageTemplateFile", "templates/template.html")
	config.SetDefault("ResourcePath", "res")
	config.SetDefault("StyleFile", "res/style.css")
	err := config.ReadInConfig()
	if err != nil && !strings.Contains(err.Error(), "Not Found") {
		checkError(err)
	}
	return config.AllSettings()
}

func createDirs(conf map[string]interface{}) {
	destPath := conf["destinationpath"].(string)
	err := os.Mkdir(destPath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}

	imagePath := path.Join(conf["destinationpath"].(string), "images")
	err = os.Mkdir(imagePath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}

	resPath := path.Join(destPath, "res")
	err = os.Mkdir(resPath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}
}

func renderPages(conf map[string]interface{}) {
	pagesmd, err := filepath.Glob(filepath.Join(conf["sourcepath"].(string), "*.md"))
	checkError(err)
	postfiles, err := filepath.Glob(filepath.Join(conf["sourcepath"].(string), "posts", "*.md"))
	checkError(err)

	pagesmd = append(pagesmd, postfiles...)

	sitename := conf["sitename"].(string)
	var data templateData

	data.SiteName = template.HTML(sitename)
	// stylefile := conf["stylefile"].(string)

	npages := len(pagesmd)
	pagelist := make([]string, npages)
	data.Body = make([]template.HTML, 1)
	plural := func(n int) string {
		if n != 1 {
			return "s"
		}
		return ""
	}

	destPath := conf["destinationpath"].(string)
	templateFile := conf["pagetemplatefile"].(string)
	fmt.Printf("Rendering %d page%s\n", npages, plural(npages))
	for idx, fname := range pagesmd {
		fmt.Printf("%d: %s", idx+1, fname)
		pagemd, err := ioutil.ReadFile(fname)
		checkError(err)

		unsafe := blackfriday.MarkdownCommon(pagemd)
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		// reverse render posts
		// data.Body[nposts-idx-1] = template.HTML(string(safe))
		data.Body[0] = template.HTML(string(safe))

		outName := fmt.Sprintf("%s.html", filenameNoExt(fname))
		outPath := filepath.Join(destPath, outName)
		err = ioutil.WriteFile(outPath, makeHTML(data, templateFile), 0666)
		checkError(err)

		fmt.Printf(" → %s\n", outPath)
		pagelist[idx] = outPath
	}
	// outPath := filepath.Join(destPath, "posts.html")
	// fmt.Printf("Saving posts: %s\n", outPath)
	// err = ioutil.WriteFile(outPath, makeHTML(data, templateFile), 0666)
	// checkError(err)
	fmt.Print("Rendering complete.\n\n")
}

// copyResources copies all files from the configured resource directory
// to the "res" subdirectory under the destination path.
func copyResources(conf map[string]interface{}) {
	fmt.Println("\nCopying resources")
	dstroot := conf["destinationpath"].(string)
	walker := func(srcloc string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("%s → %s\n", srcloc, dstloc)
			copyFile(srcloc, dstloc)
		}
		return nil
	}

	err := filepath.Walk(conf["resourcepath"].(string), walker)
	checkError(err)
	fmt.Println("Done!")
}

func main() {
	conf := loadConfig()
	createDirs(conf)
	renderPages(conf)
	getAvatar(conf)
	copyResources(conf)
}
