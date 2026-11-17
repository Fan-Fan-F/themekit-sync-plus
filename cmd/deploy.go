package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"sync"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/Shopify/themekit/src/cmdutil"
	"github.com/Shopify/themekit/src/colors"
	"github.com/Shopify/themekit/src/file"
	"github.com/Shopify/themekit/src/shopify"
)

const settingsDataKey = "config/settings_data.json"

var compiledFilenameWarning = template.Must(template.New("compiledFilenamesWarning").Parse(
	`[{{.EnvName}}] You have file names that will conflict with each other.
If you have files named [filename].js.liquid or [filename].scss.liquid,
they will be compiled to [filename].js and [filename].scss respectively
when they are uploaded to Shopify. Having both files uploaded to Shopify
will overwrite one or the other and cause unexpected behavior.

The files you will need to change are:
  {{- range .FileNames }}
	{{ . }}
	{{- end }}

To fix this, you will need to ignore, rename or delete one or both of
the files.
`))

var deployCmd = &cobra.Command{
	Use:   "deploy <filenames>",
	Short: "deploy files to shopify",
	Long: `Deploy will overwrite specific files if provided with file names.
 If deploy is not provided with file names then it will deploy all
 the files on shopify with your local files. Any files that do not
 exist on your local machine will be removed from shopify unless the --nodelete
 flag is passed

 For more information, refer to https://shopify.dev/tools/theme-kit/command-reference#deploy.
 `,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdutil.ForEachClient(flags, args, deploy)
	},
	PostRun: func(cmd *cobra.Command, args []string) {
	},
}

func deploy(ctx *cmdutil.Ctx) error {
	if ctx.Env.ReadOnly {
		return fmt.Errorf("[%s] environment is readonly", colors.Green(ctx.Env.Name))
	}

	if ctx.Flags.DryRun {
		colors.ColorStdOut.Printf("[%s] %s - No changes will be applied to remote store",
			colors.Yellow(ctx.Env.Name),
			colors.Yellow("[DRY-RUN]"),
		)
	}

	cache := shopify.NewChecksumCache(ctx.Env.Directory, ctx.Flags.NoCache)
	assetsActions, err := generateActionsWithCache(ctx, cache)
	if err != nil {
		return err
	}

	type actionItem struct {
		path string
		op   file.Op
	}

	var regularActions []actionItem
	hasSettingsData := false
	var settingsOp file.Op

	for path, op := range assetsActions {
		if path == settingsDataKey {
			hasSettingsData = true
			settingsOp = op
		} else {
			regularActions = append(regularActions, actionItem{path: path, op: op})
		}
	}

	concurrency := ctx.Flags.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	if concurrency > 50 {
		concurrency = 50
	}

	ctx.StartProgress(len(assetsActions))

	jobs := make(chan actionItem, len(regularActions))
	for _, act := range regularActions {
		jobs <- act
	}
	close(jobs)

	numWorkers := concurrency
	if len(regularActions) < numWorkers {
		numWorkers = len(regularActions)
	}
	if numWorkers <= 0 && len(regularActions) > 0 {
		numWorkers = 1
	}

	var deployGroup sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		deployGroup.Add(1)
		go func() {
			defer deployGroup.Done()
			for item := range jobs {
				perform(ctx, item.path, item.op, "")
			}
		}()
	}

	deployGroup.Wait()

	// Sync settings_data.json last as required by Shopify theme architecture
	if hasSettingsData {
		perform(ctx, settingsDataKey, settingsOp, "")
	}

	if !ctx.Flags.DryRun && !ctx.Flags.NoCache {
		_ = cache.Save()
	}

	return nil
}

func generateActions(ctx *cmdutil.Ctx) (map[string]file.Op, error) {
	cache := shopify.NewChecksumCache(ctx.Env.Directory, ctx.Flags.NoCache)
	return generateActionsWithCache(ctx, cache)
}

func generateActionsWithCache(ctx *cmdutil.Ctx, cache *shopify.ChecksumCache) (map[string]file.Op, error) {
	assetsActions := map[string]file.Op{}
	pathsToChecksums := map[string]string{}

	remoteFiles, err := ctx.Client.GetAllAssets()
	if err != nil {
		return assetsActions, err
	}
	for _, remoteAsset := range remoteFiles {
		if len(ctx.Args) == 0 && !ctx.Flags.NoDelete {
			assetsActions[remoteAsset.Key] = file.Remove
		}
		pathsToChecksums[remoteAsset.Key] = remoteAsset.Checksum
	}

	localAssets, err := shopify.FindAssetsWithCache(ctx.Env, cache, ctx.Args...)
	if err != nil {
		return assetsActions, err
	}

	problemAssets := compileAssetFilenames(localAssets)
	if len(problemAssets) > 0 {
		return assetsActions, compiledAssetWarning(ctx.Env.Name, problemAssets)
	}

	for _, asset := range localAssets {
		var path = asset.Key
		if asset.Checksum != "" && (asset.Checksum == pathsToChecksums[asset.Key]) {
			assetsActions[path] = file.Skip
		} else {
			assetsActions[path] = file.Update
		}
	}
	return assetsActions, nil
}

func compileAssetFilenames(assets []shopify.Asset) (problemAssets []string) {
	var filenames []string
	for _, asset := range assets {
		filenames = append(filenames, asset.Key)
	}
	sort.Strings(filenames)
	for i, filename := range filenames {
		if i < len(filenames)-1 && filename+".liquid" == filenames[i+1] {
			problemAssets = append(problemAssets, colors.Yellow(filenames[i])+
				colors.Blue(" conflicts with ")+
				colors.Yellow(filenames[i+1]))
		}
	}
	return
}

func compiledAssetWarning(env string, filenames []string) error {
	var tpl bytes.Buffer
	compiledFilenameWarning.Execute(&tpl, struct {
		EnvName   string
		FileNames []string
	}{EnvName: colors.Yellow(env), FileNames: filenames})
	return errors.New(tpl.String())
}
