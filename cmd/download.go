package cmd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/spf13/cobra"

	"github.com/Shopify/themekit/src/cmdutil"
	"github.com/Shopify/themekit/src/colors"
	"github.com/Shopify/themekit/src/file"
	"github.com/Shopify/themekit/src/shopify"
)

var downloadCmd = &cobra.Command{
	Use:   "download <filenames>",
	Short: "Download one or all of the theme files",
	Long: `Download will download specific files from shopify servers if provided file names.
 If no filenames are provided then download will download every file in the project
 and write them to disk.

 For more information, refer to https://shopify.dev/tools/theme-kit/command-reference#download.
 `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// download should not care about the live theme
		flags.AllowLive = true
		if flags.Live {
			theme, err := getLiveTheme(flags, args)
			if err != nil {
				return err
			}
			flags.ThemeID = strconv.Itoa(int(theme.ID))
		}
		return cmdutil.ForEachClient(flags, args, download)
	},
}

func download(ctx *cmdutil.Ctx) error {
	assets, err := filesToDownload(ctx)
	if err != nil {
		return err
	}

	if len(assets) == 0 {
		return fmt.Errorf("No files to download")
	}

	if ctx.Flags.DryRun {
		colors.ColorStdOut.Printf("[%s] %s - No files will be written to disk",
			colors.Yellow(ctx.Env.Name),
			colors.Yellow("[DRY-RUN]"),
		)
	}

	type downloadItem struct {
		path string
		op   file.Op
	}
	var items []downloadItem
	for asset, op := range assets {
		items = append(items, downloadItem{path: asset, op: op})
	}

	concurrency := ctx.Flags.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	if concurrency > 50 {
		concurrency = 50
	}

	ctx.StartProgress(len(items))

	jobs := make(chan downloadItem, len(items))
	for _, it := range items {
		jobs <- it
	}
	close(jobs)

	numWorkers := concurrency
	if len(items) < numWorkers {
		numWorkers = len(items)
	}
	if numWorkers <= 0 && len(items) > 0 {
		numWorkers = 1
	}

	var downloadGroup sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		downloadGroup.Add(1)
		go func() {
			defer downloadGroup.Done()
			for it := range jobs {
				perform(ctx, it.path, it.op, "")
			}
		}()
	}

	downloadGroup.Wait()

	return nil
}

func filesToDownload(ctx *cmdutil.Ctx) (map[string]file.Op, error) {
	fetchableFiles := map[string]file.Op{}

	assets, err := ctx.Client.GetAllAssets()
	if err != nil {
		return fetchableFiles, err
	}

	if len(ctx.Args) <= 0 {
		for _, asset := range assets {
			fetchableFiles[asset.Key] = downloadFileAction(ctx, asset)
		}
		return fetchableFiles, nil
	}

	for _, asset := range assets {
		for _, pattern := range ctx.Args {
			// These need to be converted to platform specific because filepath.Match
			// uses platform specific separators
			pattern = filepath.FromSlash(pattern)
			filename := filepath.FromSlash(asset.Key)
			globMatched, _ := filepath.Match(pattern, filename)
			dirMatched, _ := filepath.Match(pattern+string(filepath.Separator)+"*", filename)
			fileMatched := filename == pattern
			if globMatched || dirMatched || fileMatched {
				fetchableFiles[asset.Key] = downloadFileAction(ctx, asset)
			}
		}
	}

	if len(fetchableFiles) == 0 {
		return fetchableFiles, fmt.Errorf("No file paths matched the inputted arguments")
	}

	return fetchableFiles, nil
}

func downloadFileAction(ctx *cmdutil.Ctx, asset shopify.Asset) file.Op {
	op := file.Get
	if asset.Checksum == "" {
		return op
	}
	if localAsset, _ := shopify.ReadAsset(ctx.Env, asset.Key); asset.Checksum == localAsset.Checksum {
		op = file.Skip
	}
	return op
}
