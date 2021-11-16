// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	wiki_service "code.gitea.io/gitea/services/wiki"

	"github.com/stretchr/testify/assert"
)

const content = "Wiki contents for unit tests"
const message = "Wiki commit message for unit tests"

func wikiEntry(t *testing.T, repo *models.Repository, wikiName string) *git.TreeEntry {
	wikiRepo, err := git.OpenRepository(repo.WikiPath())
	assert.NoError(t, err)
	defer wikiRepo.Close()
	commit, err := wikiRepo.GetBranchCommit("master")
	assert.NoError(t, err)
	entries, err := commit.ListEntries()
	assert.NoError(t, err)
	for _, entry := range entries {
		if entry.Name() == wiki_service.NameToFilename(wikiName) {
			return entry
		}
	}
	return nil
}

func wikiContent(t *testing.T, repo *models.Repository, wikiName string) string {
	entry := wikiEntry(t, repo, wikiName)
	if !assert.NotNil(t, entry) {
		return ""
	}
	reader, err := entry.Blob().DataAsync()
	assert.NoError(t, err)
	defer reader.Close()
	bytes, err := io.ReadAll(reader)
	assert.NoError(t, err)
	return string(bytes)
}

func assertWikiExists(t *testing.T, repo *models.Repository, wikiName string) {
	assert.NotNil(t, wikiEntry(t, repo, wikiName))
}

func assertWikiNotExists(t *testing.T, repo *models.Repository, wikiName string) {
	assert.Nil(t, wikiEntry(t, repo, wikiName))
}

func assertPagesMetas(t *testing.T, expectedNames []string, metas interface{}) {
	pageMetas, ok := metas.([]PageMeta)
	if !assert.True(t, ok) {
		return
	}
	if !assert.Len(t, pageMetas, len(expectedNames)) {
		return
	}
	for i, pageMeta := range pageMetas {
		assert.EqualValues(t, expectedNames[i], pageMeta.Name)
	}
}

func TestWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/?action=_pages")
	ctx.SetParams("*", "Home")
	test.LoadRepo(t, ctx, 1)
	Wiki(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, "Home", ctx.Data["Title"])
	assertPagesMetas(t, []string{"Home", "Page With Image", "Page With Spaced Name", "Unescaped File"}, ctx.Data["Pages"])
}

func TestWikiPages(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/?action=_pages")
	test.LoadRepo(t, ctx, 1)
	WikiPages(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assertPagesMetas(t, []string{"Home", "Page With Image", "Page With Spaced Name", "Unescaped File"}, ctx.Data["Pages"])
}

func TestNewWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/?action=_new")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	NewWiki(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, ctx.Tr("repo.wiki.new_page"), ctx.Data["Title"])
}

func TestNewWikiPost(t *testing.T) {
	for _, title := range []string{
		"New page",
		"&&&&",
	} {
		unittest.PrepareTestEnv(t)

		ctx := test.MockContext(t, "user2/repo1/wiki/?action=_new")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		web.SetForm(ctx, &forms.NewWikiForm{
			Title:   title,
			Content: content,
			Message: message,
		})
		NewWikiPost(ctx)
		assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
		assertWikiExists(t, ctx.Repo.Repository, title)
		assert.Equal(t, wikiContent(t, ctx.Repo.Repository, title), content)
	}
}

func TestNewWikiPost_ReservedName(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/?action=_new")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.NewWikiForm{
		Title:   "_edit",
		Content: content,
		Message: message,
	})
	NewWikiPost(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, ctx.Tr("repo.wiki.reserved_page"), ctx.Flash.ErrorMsg)
	assertWikiNotExists(t, ctx.Repo.Repository, "_edit")
}

func TestEditWiki(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/Home?action=_edit")
	ctx.SetParams("*", "Home")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	EditWiki(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assert.EqualValues(t, "Home", ctx.Data["Title"])
	assert.Equal(t, wikiContent(t, ctx.Repo.Repository, "Home"), ctx.Data["content"])
}

func TestEditWikiPost(t *testing.T) {
	for _, title := range []string{
		"Home",
		"New/<page>",
	} {
		unittest.PrepareTestEnv(t)
		ctx := test.MockContext(t, "user2/repo1/wiki/Home?action=_new")
		ctx.SetParams("*", "Home")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		web.SetForm(ctx, &forms.NewWikiForm{
			Title:   title,
			Content: content,
			Message: message,
		})
		EditWikiPost(ctx)
		assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
		assertWikiExists(t, ctx.Repo.Repository, title)
		assert.Equal(t, wikiContent(t, ctx.Repo.Repository, title), content)
		if title != "Home" {
			assertWikiNotExists(t, ctx.Repo.Repository, "Home")
		}
	}
}

func TestDeleteWikiPagePost(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/wiki/Home?action=_delete")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	DeleteWikiPagePost(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	assertWikiNotExists(t, ctx.Repo.Repository, "Home")
}

func TestWikiRaw(t *testing.T) {
	for filepath, filetype := range map[string]string{
		"jpeg.jpg":                 "image/jpeg",
		"images/jpeg.jpg":          "image/jpeg",
		"Page With Spaced Name":    "text/plain; charset=utf-8",
		"Page-With-Spaced-Name":    "text/plain; charset=utf-8",
		"Page With Spaced Name.md": "text/plain; charset=utf-8",
		"Page-With-Spaced-Name.md": "text/plain; charset=utf-8",
	} {
		unittest.PrepareTestEnv(t)

		ctx := test.MockContext(t, "user2/repo1/wiki/raw/"+filepath)
		ctx.SetParams("*", filepath)
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		WikiRaw(ctx)
		assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
		assert.EqualValues(t, filetype, ctx.Resp.Header().Get("Content-Type"))
	}
}
