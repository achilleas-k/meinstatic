module github.com/achilleas-k/meinstatic

go 1.12

require (
	github.com/microcosm-cc/bluemonday v1.0.2
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/spf13/viper v1.4.0
	gopkg.in/russross/blackfriday.v2 v2.0.1
)

replace gopkg.in/russross/blackfriday.v2 v2.0.1 => github.com/russross/blackfriday/v2 v2.0.1
