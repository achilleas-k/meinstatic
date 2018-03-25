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
	checkError(err)
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
	// TODO: Handle relative paths in template (for res stuff)
	thtml := readTemplate(templateFile)
	t, err := template.New("webpage").Parse(thtml)
	checkError(err)
	rendered := new(bytes.Buffer)
	err = t.Execute(rendered, data)
	checkError(err)
	return rendered.Bytes()
}

func loadConfig() map[string]interface{} {
	config := viper.GetViper()
	config.SetConfigName("config")
	config.AddConfigPath(".")
	config.SetDefault("SiteName", "")
	config.SetDefault("SourcePath", "pages-md")
	config.SetDefault("DestinationPath", "html")
	config.SetDefault("GravatarUsername", "")
	config.SetDefault("GravatarEmail", "")
	config.SetDefault("PageTemplateFile", "templates/template.html")
	config.SetDefault("ResourcePath", "res")
	err := config.ReadInConfig()
	if err != nil && !strings.Contains(err.Error(), "Not Found") {
		checkError(err)
	}
	return config.AllSettings()
}

func createDirs(conf map[string]interface{}) {
	destpath := conf["destinationpath"].(string)
	err := os.Mkdir(destpath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}

	imagepath := path.Join(conf["destinationpath"].(string), "images")
	err = os.Mkdir(imagepath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}

	respath := path.Join(destpath, "res")
	err = os.Mkdir(respath, 0777)
	if !os.IsExist(err) {
		checkError(err)
	}
}

func renderPages(conf map[string]interface{}) {
	srcpath := conf["sourcepath"].(string)

	var pagesmd []string
	mdfinder := func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".md" {
			pagesmd = append(pagesmd, path)
		}
		return nil
	}
	filepath.Walk(srcpath, mdfinder)

	sitename := conf["sitename"].(string)
	var data templateData

	data.SiteName = template.HTML(sitename)

	npages := len(pagesmd)
	pagelist := make([]string, npages)
	data.Body = make([]template.HTML, 1)
	plural := func(n int) string {
		if n != 1 {
			return "s"
		}
		return ""
	}

	destpath := conf["destinationpath"].(string)
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

		// trim source path
		outpath := strings.TrimPrefix(fname, srcpath)
		// trim extension (and replace with .html)
		outpath = strings.TrimSuffix(outpath, ".md")
		outpath = fmt.Sprintf("%s.html", outpath)
		outpath = filepath.Join(destpath, outpath)

		// make potential parent directory
		outpathpar, _ := filepath.Split(outpath)
		if outpathpar != destpath {
			os.MkdirAll(outpathpar, 0777)
		}

		err = ioutil.WriteFile(outpath, makeHTML(data, templateFile), 0666)
		checkError(err)

		fmt.Printf(" → %s\n", outpath)
		pagelist[idx] = outpath
	}
	// outpath := filepath.Join(destpath, "posts.html")
	// fmt.Printf("Saving posts: %s\n", outpath)
	// err = ioutil.WriteFile(outpath, makeHTML(data, templateFile), 0666)
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
		// TODO: Skip hidden files and dirs
		if info.Mode().IsRegular() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("%s → %s\n", srcloc, dstloc)
			copyFile(srcloc, dstloc)
		} else if info.Mode().IsDir() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("Creating directory %s\n", dstloc)
			os.Mkdir(dstloc, 0777)
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
