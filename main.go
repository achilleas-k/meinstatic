package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/spf13/viper"
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
	data, err := os.ReadFile(srcName)
	checkError(err)
	return os.WriteFile(dstName, data, 0666)
}

func readTemplate(templateFile string) string {
	thtml, err := os.ReadFile(templateFile)
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
	config.SetDefault("PageTemplateFile", "templates/template.html")
	config.SetDefault("ResourcePath", "res")
	config.SetDefault("PostPattern", `[0-9]{8}-.*`)
	err := config.ReadInConfig()
	if err != nil && !strings.Contains(err.Error(), "Not Found") {
		checkError(err)
	}
	return config.AllSettings()
}

func createDirs(conf map[string]interface{}) {
	destpath := conf["destinationpath"].(string)
	checkError(os.MkdirAll(destpath, 0777))

	imagepath := path.Join(conf["destinationpath"].(string), "images")
	checkError(os.MkdirAll(imagepath, 0777))

	respath := path.Join(destpath, "res")
	checkError(os.MkdirAll(respath, 0777))
}

type post struct {
	date    string // TODO: Change to datetime
	title   string
	summary string
	url     string
}

// childLiterals concatenates the literals under a given node into a single
// string.
func childLiterals(node *blackfriday.Node) string {
	var lit string
	for child := node.FirstChild; child != nil; child = child.Next {
		if child.IsLeaf() {
			lit += string(child.Literal)
		} else {
			lit += childLiterals(child)
		}
	}
	return lit
}

func parsePost(mdsource []byte) (p post) {
	md := blackfriday.New()
	rootnode := md.Parse(mdsource)
	visitor := func(node *blackfriday.Node, _ bool) blackfriday.WalkStatus {
		if node.Type == blackfriday.Heading && node.Level == 1 && p.title == "" {
			p.title = childLiterals(node)
		} else if node.Type == blackfriday.Paragraph {
			// Found first paragraph
			p.summary = childLiterals(node)
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
	mdfinder := func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".md" {
			pagesmd = append(pagesmd, path)
		}
		return nil
	}
	checkError(filepath.Walk(srcpath, mdfinder))

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
	postrePattern := conf["postpattern"].(string)
	fmt.Printf(":: Rendering %d page%s\n", npages, plural(npages))
	postre, err := regexp.Compile(postrePattern)
	checkError(err)
	for idx, fname := range pagesmd {
		fmt.Printf("   %d: %s", idx+1, fname)
		pagemd, err := os.ReadFile(fname)
		checkError(err)

		bfext := blackfriday.WithExtensions(blackfriday.CommonExtensions | blackfriday.AutoHeadingIDs)
		unsafe := blackfriday.Run(pagemd, bfext)
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
			checkError(os.MkdirAll(outpathpar, 0777))
		}
		data.RelRoot, _ = filepath.Rel(outpathpar, destpath)

		checkError(os.WriteFile(outpath, makeHTML(data, templateFile), 0666))

		if postre.MatchString(fname) {
			p := parsePost(pagemd)
			postURL := strings.TrimPrefix(outpath, destpath)
			postURL = strings.TrimPrefix(postURL, "/") // make it relative
			p.url = postURL
			postlisting = append(postlisting, p)
			nposts++
		}

		fmt.Printf(" -> %s\n", outpath)
		pagelist[idx] = outpath
	}
	fmt.Printf(":: Found %d posts\n", nposts)

	// render to listing page
	if nposts > 0 {
		// TODO: create listing page as ast instead of manually rendering blocks
		var bodystr string
		for idx, p := range postlisting {
			bodystr = fmt.Sprintf("%s%d. [%s](%s)\n    - %s\n", bodystr, idx, p.title, p.url, p.summary)
		}
		unsafe := blackfriday.Run([]byte(bodystr))
		safe := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
		data.Body = template.HTML(string(safe))
		outpath := filepath.Join(destpath, "posts.html")
		fmt.Printf("   Saving posts: %s\n", outpath)
		err := os.WriteFile(outpath, makeHTML(data, templateFile), 0666)
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
			checkError(copyFile(srcloc, dstloc))
		} else if info.Mode().IsDir() {
			dstloc := path.Join(dstroot, srcloc)
			fmt.Printf("   Creating directory %s\n", dstloc)
			checkError(os.MkdirAll(dstloc, 0777))
		}
		return nil
	}

	checkError(filepath.Walk(conf["resourcepath"].(string), walker))
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
	copyResources(conf)
}
