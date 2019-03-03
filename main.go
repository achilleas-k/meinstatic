package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/spf13/viper"
	"gopkg.in/russross/blackfriday.v2"
)

var (
	build  string
	commit string
	verstr string
)

type templateData struct {
	SiteName template.HTML
	Body     template.HTML
	// RelRoot is a relative path prefix that points to the root of the HTML destination directory.
	// It can be used to make relative links to pages and resources.
	RelRoot string
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

type post struct {
	date    string // TODO: Change to datetime
	title   string
	summary string
	url     string
}

func parsePost(mdsource []byte) (p post) {
	md := blackfriday.New()
	rootnode := md.Parse(mdsource)
	visitor := func(node *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if node.Parent != nil && node.Parent.Type == blackfriday.Heading && node.Parent.Level == 1 && p.title == "" {
			p.title = string(node.Literal)
		} else if node.Parent != nil && node.Parent.Type == blackfriday.Paragraph {
			// Found first paragraph
			p.summary = string(node.Literal)
			return blackfriday.Terminate
		}
		return blackfriday.GoToNext
	}
	rootnode.Walk(visitor)
	return
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
	plural := func(n int) string {
		if n != 1 {
			return "s"
		}
		return ""
	}

	nposts := 0
	postlisting := make([]post, 0, npages)

	destpath := conf["destinationpath"].(string)
	templateFile := conf["pagetemplatefile"].(string)
	fmt.Printf(":: Rendering %d page%s\n", npages, plural(npages))
	for idx, fname := range pagesmd {
		fmt.Printf("   %d: %s", idx+1, fname)
		pagemd, err := ioutil.ReadFile(fname)
		checkError(err)

		unsafe := blackfriday.Run(pagemd)
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		// reverse render posts
		// data.Body[nposts-idx-1] = template.HTML(string(safe))
		data.Body = template.HTML(string(safe))

		// trim source path
		outpath := strings.TrimPrefix(fname, srcpath)
		// trim extension (and replace with .html)
		outpath = strings.TrimSuffix(outpath, filepath.Ext(outpath))
		outpath = fmt.Sprintf("%s.html", outpath)
		outpath = filepath.Join(destpath, outpath)

		// make potential parent directory
		outpathpar, _ := filepath.Split(outpath)
		if outpathpar != destpath {
			os.MkdirAll(outpathpar, 0777)
		}
		data.RelRoot, _ = filepath.Rel(outpathpar, destpath)

		err = ioutil.WriteFile(outpath, makeHTML(data, templateFile), 0666)
		checkError(err)

		if strings.Contains(fname, "post") {
			p := parsePost(pagemd)
			p.url = strings.TrimPrefix(outpath, destpath)
			postlisting = append(postlisting, p)
			nposts++
		}

		fmt.Printf(" -> %s\n", outpath)
		pagelist[idx] = outpath
	}
	fmt.Printf(":: Found %d posts\n", nposts)
	// render to listing page

	if nposts > 0 {
		var bodystr string
		for idx, p := range postlisting {
			bodystr = fmt.Sprintf("%s%d. [%s](%s) %s\n", bodystr, idx, p.title, p.url, p.summary)
		}
		unsafe := blackfriday.Run([]byte(bodystr))
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		data.Body = template.HTML(string(safe))
		outpath := filepath.Join(destpath, "posts.html")
		fmt.Printf("   Saving posts: %s\n", outpath)
		err := ioutil.WriteFile(outpath, makeHTML(data, templateFile), 0666)
		checkError(err)
	}
	fmt.Println(":: Rendering complete!")
}

// copyResources copies all files from the configured resource directory
// to the "res" subdirectory under the destination path.
func copyResources(conf map[string]interface{}) {
	fmt.Println(":: Copying resources")
	dstroot := conf["destinationpath"].(string)
	walker := func(srcloc string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("   %s -> %s\n", srcloc, dstloc)
			copyFile(srcloc, dstloc)
		} else if info.Mode().IsDir() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("   Creating directory %s\n", dstloc)
			os.Mkdir(dstloc, 0777)
		}
		return nil
	}

	err := filepath.Walk(conf["resourcepath"].(string), walker)
	checkError(err)
	fmt.Println("== Done ==")
}

func printversion() {
	fmt.Println(verstr)
}

func init() {
	if build == "" {
		verstr = "meinstatic [dev build]"
	} else {
		verstr = fmt.Sprintf("meinstatic Build %s (%s)", build, commit)
	}
}

func main() {
	var printver bool
	flag.BoolVar(&printver, "version", false, "print version number")
	flag.Parse()
	if printver {
		printversion()
		return
	}
	conf := loadConfig()
	createDirs(conf)
	renderPages(conf)
	getAvatar(conf)
	copyResources(conf)
}
