package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/urfave/cli/v3"
)

var (
	ARGS_USAGE_SHOWDEPS  = "[options] [image names...]"
	DESCRIPTION_SHOWDEPS = `Checks and shows dependencies between images.
	This command defines a subcommand showdeps that checks and displays
	the dependencies between Docker images as a Mermaid flowchart.
	Here's a short description of the command's key components:

	Examples)
	#> gdocker showdeps
	#> gdocker showdeps --gfm`
)

var (
	TMPL_MERMAID = `{{< if .GFM >}}` + "```mermaid" + `{{< end >}}
flowchart TD

    classDef root fill:#8BA7D5,color:#000000
    classDef latest fill:#E38692,color:#000000
    classDef latestimg fill:#F6D580,color:#000000
    classDef old fill:#81D674,color:#000000
{{< range .Deps >}}
    {{< . >}}{{< end >}}
{{< if .GFM >}}` + "```" + `{{< end >}}
`
	TMPL_MERMAID_WEB = `
<script type="module">import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs'</script>

<pre class="mermaid">
	flowchart TD

    classDef root fill:#8BA7D5,color:#000000
    classDef latest fill:#E38692,color:#000000
    classDef latestimg fill:#F6D580,color:#000000
    classDef old fill:#81D674,color:#000000
	{{ range . }}
    {{ . }}{{ end }}
</pre>
`
)

func cmdShowDeps() *cli.Command {
	return &cli.Command{
		Name:               "showdeps",
		Usage:              "show docker image dependencies as mermaid flowchart",
		CustomHelpTemplate: TMPL_SUBCOMMAND_HELP,
		ArgsUsage:          ARGS_USAGE_SHOWDEPS,
		Description:        DESCRIPTION_SHOWDEPS,
		Before:             setSubCommandHelpTemplate(TMPL_SUBCOMMAND_HELP),
		Flags: []cli.Flag{
			FLAG_DIRECTORY,
			FLAG_LIST,
			FLAG_ALL,
			FLAG_ALL_LATEST,
			FLAG_GFM,
			FLAG_WEB,
			FLAG_CONFIG_DEFAULT,
			FLAG_VERBOSE,
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			logger := getLogger("showdeps", getLogLevel(cmd.Int("verbose")))
			slog.SetDefault(logger)

			config, _ := loadConfig(cmd)
			dir := config.Dir

			ibds := searchImageBuildDir(dir, "archive")
			ibds.makeMap()
			deps := ibds.Dependencies()

			inputs := checkImageNamesInput(cmd, ibds) // load input image names from -l and args

			var images []DockerImage
			for _, input := range inputs {
				img, err := NewDockerImage(input)
				if err != nil {
					slog.Error(err.Error())
					os.Exit(1)
				}
				if _, ok := ibds.mapNameTag[img.String()]; !ok {
					slog.Warn(fmt.Sprintf("%v is not found. skipped.", img))
					continue
				}
				images = append(images, img)
			}
			solved, roots := checkDependency(images, deps)

			var deps_sub []Dependency
			for _, img := range solved {
				for _, dep := range deps {
					if img.String() == dep.From.String() {
						if _, ok := roots[dep.To.String()]; ok {
							dep.To.IsRoot = true
						}
						deps_sub = append(deps_sub, dep)
					}
				}
			}

			type tmplData struct {
				GFM  bool
				Deps []Dependency
			}

			tmpl := NewTemplates(TMPL_MERMAID, tmplData{
				cmd.Bool("gfm"),
				deps_sub,
			})
			tmpl.writeTemplates("stdout", false)

			if cmd.Bool("web") {
				http.HandleFunc("/", viewHandler(deps_sub))
				slog.Warn("Server started url http://localhost:8080/")
				log.Fatal(http.ListenAndServe("localhost:8080", nil))
			}
			return nil
		},
	}
}

func (dep Dependency) String() string {
	if dep.From.Tag == "latest" {
		dep.To.IsLatest = true
	}
	return fmt.Sprintf("%s --> %s", printNode(dep.From), printNode(dep.To))
}

func printNode(di DockerImage) string {
	if di.IsRoot {
		return fmt.Sprintf(`%s[["%s [root]"]]:::root`, di.String(), di.String())
	}
	if di.Tag == "latest" {
		return fmt.Sprintf(`%s("%s"):::latest`, di.String(), di.String())
	}
	if di.IsLatest {
		return fmt.Sprintf(`%s("%s"):::latestimg`, di.String(), di.String())
	}
	return fmt.Sprintf(`%s("%s"):::old`, di.String(), di.String())
}

func viewHandler(deps any) func(w http.ResponseWriter, r *http.Request) {
	f := func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.New("mermaid").Parse(TMPL_MERMAID_WEB)
		if err != nil {
			http.Error(w, "template parse error", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, deps); err != nil {
			http.Error(w, "template execute error", http.StatusInternalServerError)
			return
		}
	}
	return f
}
