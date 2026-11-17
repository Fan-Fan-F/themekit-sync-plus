package cmd

import (
	"bytes"
	"log"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Shopify/themekit/src/cmdutil"
	mocks "github.com/Shopify/themekit/src/cmdutil/_mocks"
	"github.com/Shopify/themekit/src/env"
	"github.com/Shopify/themekit/src/file"
	"github.com/Shopify/themekit/src/shopify"
)

func TestPerform_DryRun(t *testing.T) {
	stdOut := bytes.NewBufferString("")
	stdErr := bytes.NewBufferString("")
	client := new(mocks.ShopifyClient)

	ctx := &cmdutil.Ctx{
		Env:    &env.Env{Name: "staging", Directory: "_testdata/projectdir"},
		Flags:  cmdutil.Flags{DryRun: true, Verbose: true},
		Client: client,
		Log:    log.New(stdOut, "", 0),
		ErrLog: log.New(stdErr, "", 0),
	}

	// Should not call DeleteAsset on Client
	perform(ctx, "assets/test.js", file.Remove, "")
	assert.Contains(t, stdOut.String(), "[DRY-RUN] Would delete assets/test.js")

	// Should not call UpdateAsset on Client
	perform(ctx, "assets/test.js", file.Update, "")
	assert.Contains(t, stdOut.String(), "[DRY-RUN] Would update assets/test.js")

	// Should not call GetAsset on Client
	perform(ctx, "assets/test.js", file.Get, "")
	assert.Contains(t, stdOut.String(), "[DRY-RUN] Would download assets/test.js")

	// No calls should have been made to the mock client
	client.AssertNotCalled(t, "DeleteAsset", mock.Anything)
	client.AssertNotCalled(t, "UpdateAsset", mock.Anything, mock.Anything)
	client.AssertNotCalled(t, "GetAsset", mock.Anything)
}

func TestGenerateActions_WithChecksumCache(t *testing.T) {
	tempDir := t.TempDir()
	cache := shopify.NewChecksumCache(tempDir, false)

	client := new(mocks.ShopifyClient)
	client.On("GetAllAssets").Return([]shopify.Asset{
		{Key: "assets/app.js", Checksum: "existing_checksum"},
	}, nil)

	ctx := &cmdutil.Ctx{
		Env:    &env.Env{Name: "production", Directory: filepath.Join("_testdata", "projectdir")},
		Flags:  cmdutil.Flags{NoDelete: true},
		Client: client,
	}

	actions, err := generateActionsWithCache(ctx, cache)
	assert.NoError(t, err)
	assert.NotNil(t, actions)
	assert.Contains(t, actions, "assets/app.js")

	// Verify that cache recorded local files
	assert.True(t, cache.Count() > 0)

	// Save and check file
	err = cache.Save()
	assert.NoError(t, err)
}

func TestDeploy_ConcurrencyWorkerPool(t *testing.T) {
	ctx, client, _, _, _ := createTestCtx()
	ctx.Env.Directory = filepath.Join("_testdata", "projectdir")
	ctx.Flags.Concurrency = 4
	ctx.Flags.NoDelete = true

	client.On("GetAllAssets").Return([]shopify.Asset{}, nil)
	client.On("UpdateAsset", mock.Anything, mock.Anything).Return(nil)

	err := deploy(ctx)
	assert.NoError(t, err)
}
