package image_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

func stage(t *testing.T, e env, actor user.User) (recipe.CommentImage, error) {
	t.Helper()
	return e.images.StageCommentImage(context.Background(), e.recipe.ID, actor, bytes.NewReader(pngBytes(t, 64, 64)))
}

func commentFiles(e env, id string) []string {
	dir := recipe.CommentImageDir(e.dir, e.recipe.ID, id)
	return []string{filepath.Join(dir, id+".jpg"), filepath.Join(dir, id+"_detail.jpg"), filepath.Join(dir, id+"_thumb.jpg")}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestStageCommentImageWritesVariants(t *testing.T) {
	e := setup(t)
	img, err := stage(t, e, adminOf(e))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range commentFiles(e, img.ID) {
		if !exists(p) {
			t.Errorf("missing %s", p)
		}
	}
	var commentID *int64
	var uploader *string
	if err := e.conn.QueryRow(`SELECT comment_id, uploader_id FROM comment_images WHERE id = ?`, img.ID).Scan(&commentID, &uploader); err != nil {
		t.Fatal(err)
	}
	if commentID != nil || uploader == nil || *uploader != e.owner {
		t.Errorf("comment_id = %v, uploader = %v; want NULL and %s", commentID, uploader, e.owner)
	}
	if img.Width != 64 || img.Height != 64 || img.InRecipe {
		t.Errorf("image = %+v", img)
	}
}

func TestStageCommentImageOnLockedRecipe(t *testing.T) {
	e := setup(t)
	if _, err := e.conn.Exec(`UPDATE recipes SET edit_policy = 'locked' WHERE id = ?`, e.recipe.ID); err != nil {
		t.Fatal(err)
	}
	member := user.User{ID: createUser(t, e, "mia"), Role: user.RoleUser}
	if _, err := stage(t, e, member); err != nil {
		t.Fatalf("member on locked recipe: %v", err)
	}
}

// TestStageCommentImageCap: a ninth unsent photo evicts the member's oldest,
// so failed sends never lock anyone out of staging.
func TestStageCommentImageCap(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	a := user.User{ID: createUser(t, e, "ann"), Role: user.RoleUser}
	b := user.User{ID: createUser(t, e, "ben"), Role: user.RoleUser}
	bFirst, err := stage(t, e, b)
	if err != nil {
		t.Fatal(err)
	}
	var first recipe.CommentImage
	for i := 0; i < 8; i++ {
		img, err := stage(t, e, a)
		if err != nil {
			t.Fatalf("stage %d: %v", i+1, err)
		}
		if i == 0 {
			first = img
		}
	}
	ninth, err := stage(t, e, a)
	if err != nil {
		t.Fatalf("9th: %v", err)
	}
	var n int
	if err := e.conn.QueryRow(`SELECT COUNT(*) FROM comment_images WHERE id = ?`, first.ID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("oldest row: %d, %v; want gone", n, err)
	}
	if exists(recipe.CommentImageDir(e.dir, e.recipe.ID, first.ID)) {
		t.Error("oldest photo's directory still there")
	}
	if !exists(recipe.CommentImageDir(e.dir, e.recipe.ID, ninth.ID)) {
		t.Error("ninth photo's directory missing")
	}
	if got, err := sqlc.New(e.conn).CountStagedCommentImages(ctx, &a.ID); err != nil || got != 8 {
		t.Errorf("ann staged = %d, %v; want 8", got, err)
	}
	if !exists(recipe.CommentImageDir(e.dir, e.recipe.ID, bFirst.ID)) {
		t.Error("another member's photo was evicted")
	}
	if got, err := sqlc.New(e.conn).CountStagedCommentImages(ctx, &b.ID); err != nil || got != 1 {
		t.Errorf("ben staged = %d, %v; want 1", got, err)
	}
}

func TestStageCommentImageUnknownRecipe(t *testing.T) {
	e := setup(t)
	_, err := e.images.StageCommentImage(context.Background(), "00000000-0000-7000-8000-000000000000", adminOf(e), bytes.NewReader(pngBytes(t, 64, 64)))
	if !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if exists(filepath.Join(e.dir, "00000000-0000-7000-8000-000000000000")) {
		t.Error("files left on disk")
	}
}

func TestOpenCommentImageRefusesStagedAndDetached(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	img, err := stage(t, e, adminOf(e))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.images.OpenCommentImage(ctx, e.recipe.ID, img.ID, "thumb"); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("staged = %v, want ErrNotFound", err)
	}
	if _, err := e.conn.Exec(`INSERT INTO recipe_comments (recipe_id, author_id, body, created_at) VALUES (?, ?, 'x', ?)`, e.recipe.ID, e.owner, db.FormatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	if _, err := e.conn.Exec(`UPDATE comment_images SET comment_id = (SELECT MAX(id) FROM recipe_comments) WHERE id = ?`, img.ID); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"thumb", "detail", "original"} {
		f, err := e.images.OpenCommentImage(ctx, e.recipe.ID, img.ID, v)
		if err != nil {
			t.Fatalf("attached %s: %v", v, err)
		}
		f.Close()
	}
	if _, err := e.images.OpenCommentImage(ctx, e.recipe.ID, img.ID, "huge"); !errors.Is(err, image.ErrNotFound) {
		t.Errorf("unknown variant = %v", err)
	}
	for _, bad := range [][2]string{{"..", img.ID}, {e.recipe.ID, ".."}, {e.recipe.ID, "../" + img.ID}} {
		if _, err := e.images.OpenCommentImage(ctx, bad[0], bad[1], "thumb"); !errors.Is(err, image.ErrNotFound) {
			t.Errorf("%v = %v, want ErrNotFound", bad, err)
		}
	}
	if _, err := e.conn.Exec(`DELETE FROM comment_images WHERE id = ?`, img.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.images.OpenCommentImage(ctx, e.recipe.ID, img.ID, "thumb"); !errors.Is(err, image.ErrNotFound) {
		t.Fatalf("row gone, file present = %v, want ErrNotFound", err)
	}
}

func TestSweepCommentImages(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	now := time.Now()
	member := user.User{ID: createUser(t, e, "mia"), Role: user.RoleUser}

	e.images.SetClock(func() time.Time { return now.Add(-25 * time.Hour) })
	old, _ := stage(t, e, adminOf(e))
	e.images.SetClock(func() time.Time { return now })
	orphan, _ := stage(t, e, member)
	fresh, _ := stage(t, e, adminOf(e))
	attached, _ := stage(t, e, adminOf(e))
	if _, err := e.conn.Exec(`UPDATE comment_images SET uploader_id = NULL WHERE id = ?`, orphan.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.conn.Exec(`INSERT INTO recipe_comments (recipe_id, author_id, body, created_at) VALUES (?, ?, 'x', ?)`, e.recipe.ID, e.owner, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.conn.Exec(`UPDATE comment_images SET comment_id = (SELECT MAX(id) FROM recipe_comments), created_at = ? WHERE id = ?`, db.FormatTime(now.Add(-48*time.Hour)), attached.ID); err != nil {
		t.Fatal(err)
	}

	if err := e.images.SweepCommentImages(ctx, now); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		id   string
		want bool
	}{{"old", old.ID, false}, {"orphan", orphan.ID, false}, {"fresh", fresh.ID, true}, {"attached", attached.ID, true}} {
		if got := exists(recipe.CommentImageDir(e.dir, e.recipe.ID, c.id)); got != c.want {
			t.Errorf("%s dir exists = %v, want %v", c.name, got, c.want)
		}
	}
	n, err := sqlc.New(e.conn).CountStagedCommentImages(ctx, &e.owner)
	if err != nil || n != 1 {
		t.Errorf("staged left = %d, %v; want 1", n, err)
	}
}

func TestCommentImageHandler(t *testing.T) {
	h, tokens, samID, _ := newStack(t)
	cookie := loginCookie(t, h)
	r := createRecipe(t, h, cookie)
	path := "/api/v1/recipes/" + r.ID + "/comment-images"
	body, ct := multipartBody(t, pngBytes(t, 64, 64), "image/png")

	if rec := doReq(h, http.MethodPost, path, body, ct, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", rec.Code)
	}
	raw, _, err := tokens.Create(context.Background(), samID, "ro", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 10<<20+1)
	if rec := doReq(h, http.MethodPost, path, big, ct, cookie); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("11 MiB: %d", rec.Code)
	}
	gif, gct := multipartBody(t, []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "image/png")
	if rec := doReq(h, http.MethodPost, path, gif, gct, cookie); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("gif: %d", rec.Code)
	}
	rec := doReq(h, http.MethodPost, path, body, ct, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stage: %d %s", rec.Code, rec.Body.String())
	}
	var id struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &id); err != nil {
		t.Fatal(err)
	}
	if rec := doReq(h, http.MethodGet, "/images/"+r.ID+"/comments/"+id.ID+"/thumb.jpg", nil, "", cookie); rec.Code != http.StatusNotFound {
		t.Errorf("staged GET: %d", rec.Code)
	}

	// A recipes:read token may not stage.
	treq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	treq.Host = "localhost:8060"
	treq.Header.Set("Content-Type", ct)
	treq.Header.Set("Authorization", "Bearer "+raw)
	trec := httptest.NewRecorder()
	h.ServeHTTP(trec, treq)
	if trec.Code != http.StatusForbidden {
		t.Errorf("read token: %d %s", trec.Code, trec.Body.String())
	}
}

// TestCommentImageFileErrors pins the GET split: an unknown photo is a 404,
// any other failure to open the file is a 500.
func TestCommentImageFileErrors(t *testing.T) {
	h, _, _, dir := newStack(t)
	cookie := loginCookie(t, h)
	r := createRecipe(t, h, cookie)
	body, ct := multipartBody(t, pngBytes(t, 64, 64), "image/png")
	rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/comment-images", body, ct, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stage: %d %s", rec.Code, rec.Body.String())
	}
	var staged struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &staged); err != nil {
		t.Fatal(err)
	}
	comment, _ := json.Marshal(map[string]any{"body": "Lecker", "imageIds": []string{staged.ID}})
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/comments", comment, "application/json", cookie); rec.Code != http.StatusCreated {
		t.Fatalf("comment: %d %s", rec.Code, rec.Body.String())
	}

	if rec := doReq(h, http.MethodGet, "/images/"+r.ID+"/comments/00000000-0000-7000-8000-000000000000/thumb.jpg", nil, "", cookie); rec.Code != http.StatusNotFound {
		t.Errorf("unknown photo: %d", rec.Code)
	}
	// A file where the photo's directory belongs makes os.Open fail with
	// ENOTDIR, which is not ErrNotExist.
	photoDir := recipe.CommentImageDir(dir, r.ID, staged.ID)
	if err := os.RemoveAll(photoDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(photoDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if rec := doReq(h, http.MethodGet, "/images/"+r.ID+"/comments/"+staged.ID+"/thumb.jpg", nil, "", cookie); rec.Code != http.StatusInternalServerError {
		t.Errorf("broken file: %d", rec.Code)
	}
}

// attached stages a photo as actor and sends it with a comment on e's recipe.
func attached(t *testing.T, e env, recipeID string, actor user.User) recipe.CommentImage {
	t.Helper()
	ctx := context.Background()
	img, err := e.images.StageCommentImage(ctx, recipeID, actor, bytes.NewReader(pngBytes(t, 64, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.recipes.AddComment(ctx, actor, recipeID, "Lecker", []string{img.ID}); err != nil {
		t.Fatal(err)
	}
	return img
}

func recipeFiles(e env, id string) []string {
	dir := filepath.Join(e.dir, e.recipe.ID)
	return []string{filepath.Join(dir, id+".jpg"), filepath.Join(dir, id+"_detail.jpg"), filepath.Join(dir, id+"_thumb.jpg")}
}

func inRecipe(t *testing.T, e env, id string) bool {
	t.Helper()
	comments, err := e.recipes.Comments(context.Background(), adminOf(e), e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range comments {
		for _, im := range c.Images {
			if im.ID == id {
				return im.InRecipe
			}
		}
	}
	t.Fatalf("comment photo %s not listed", id)
	return false
}

func TestAddCommentImageToRecipe(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	photo := attached(t, e, e.recipe.ID, adminOf(e))
	editor := user.User{ID: createUser(t, e, "mara"), Role: user.RoleUser}

	img, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, editor)
	if err != nil {
		t.Fatal(err)
	}
	if img.ID == photo.ID || img.Position != 0 || img.Width != 64 || img.Height != 64 {
		t.Fatalf("image = %+v", img)
	}
	r, err := e.recipes.ByID(ctx, e.recipe.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.CoverImageID == nil || *r.CoverImageID != img.ID {
		t.Errorf("cover = %v, want %s", r.CoverImageID, img.ID)
	}
	if len(r.Images) != 1 || r.Images[0].ID != img.ID {
		t.Errorf("images = %+v", r.Images)
	}
	if r.UpdatedBy.ID != editor.ID {
		t.Errorf("updatedBy = %q, want %q", r.UpdatedBy.ID, editor.ID)
	}
	if !inRecipe(t, e, photo.ID) {
		t.Error("inRecipe = false after adding")
	}
	for i, p := range commentFiles(e, photo.ID) {
		a, errA := os.Stat(p)
		b, errB := os.Stat(recipeFiles(e, img.ID)[i])
		if errA != nil || errB != nil || !os.SameFile(a, b) {
			t.Errorf("variant %d not shared: %v %v", i, errA, errB)
		}
	}

	second := attached(t, e, e.recipe.ID, adminOf(e))
	next, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, second.ID, adminOf(e))
	if err != nil {
		t.Fatal(err)
	}
	if next.Position != 1 {
		t.Errorf("second position = %d, want 1", next.Position)
	}
	if r, _ := e.recipes.ByID(ctx, e.recipe.ID); r.CoverImageID == nil || *r.CoverImageID != img.ID {
		t.Errorf("cover moved to %v", r.CoverImageID)
	}
}

func TestAddCommentImageToRecipeTwice(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	photo := attached(t, e, e.recipe.ID, adminOf(e))
	img, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e)); !errors.Is(err, image.ErrAlreadyInRecipe) {
		t.Fatalf("second add = %v, want ErrAlreadyInRecipe", err)
	}
	if err := e.images.Delete(ctx, e.recipe.ID, img.ID, adminOf(e)); err != nil {
		t.Fatal(err)
	}
	if inRecipe(t, e, photo.ID) {
		t.Error("inRecipe = true after the recipe photo was deleted")
	}
	if _, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e)); err != nil {
		t.Fatalf("add after delete: %v", err)
	}
}

func TestAddCommentImageToRecipeRules(t *testing.T) {
	ctx := context.Background()

	t.Run("locked", func(t *testing.T) {
		e := setup(t)
		photo := attached(t, e, e.recipe.ID, adminOf(e))
		ben := lockedForMember(t, e)
		if _, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, ben); !errors.Is(err, recipe.ErrEditForbidden) {
			t.Fatalf("err = %v, want ErrEditForbidden", err)
		}
		if inRecipe(t, e, photo.ID) {
			t.Error("inRecipe after a refused add")
		}
	})

	t.Run("full", func(t *testing.T) {
		e := setup(t)
		photo := attached(t, e, e.recipe.ID, adminOf(e))
		for range 20 {
			upload(t, e, 64, 64)
		}
		before, _ := os.ReadDir(filepath.Join(e.dir, e.recipe.ID))
		if _, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e)); !errors.Is(err, image.ErrTooMany) {
			t.Fatalf("err = %v, want ErrTooMany", err)
		}
		if after, _ := os.ReadDir(filepath.Join(e.dir, e.recipe.ID)); len(after) != len(before) {
			t.Errorf("files left behind: %d entries, want %d", len(after), len(before))
		}
	})

	t.Run("not attached here", func(t *testing.T) {
		e := setup(t)
		staged, err := stage(t, e, adminOf(e))
		if err != nil {
			t.Fatal(err)
		}
		other, err := e.recipes.Create(ctx, e.owner, recipe.Input{
			Title: "Anderes", Servings: 1,
			IngredientGroups: []recipe.IngredientGroup{{Ingredients: []recipe.Ingredient{{Name: "Salz"}}}},
			Steps:            []recipe.Step{{Text: "Salzen."}},
		})
		if err != nil {
			t.Fatal(err)
		}
		foreign := attached(t, e, other.ID, adminOf(e))
		for name, id := range map[string]string{"staged": staged.ID, "other recipe's": foreign.ID, "malformed": "../x"} {
			if _, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, id, adminOf(e)); !errors.Is(err, image.ErrNotFound) {
				t.Errorf("%s photo: err = %v, want ErrNotFound", name, err)
			}
		}
	})
}

func TestAddedPhotoOutlivesItsComment(t *testing.T) {
	ctx := context.Background()
	open := func(t *testing.T, f *os.File, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
	}

	t.Run("comment deleted", func(t *testing.T) {
		e := setup(t)
		photo := attached(t, e, e.recipe.ID, adminOf(e))
		img, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e))
		if err != nil {
			t.Fatal(err)
		}
		comments, err := e.recipes.Comments(ctx, adminOf(e), e.recipe.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.recipes.DeleteComment(ctx, adminOf(e), comments[0].ID); err != nil {
			t.Fatal(err)
		}
		if exists(recipe.CommentImageDir(e.dir, e.recipe.ID, photo.ID)) {
			t.Error("comment photo dir survived its comment")
		}
		for _, v := range []string{"thumb", "detail", "original"} {
			f, err := e.images.Open(e.recipe.ID, img.ID, v)
			open(t, f, err)
		}
	})

	t.Run("recipe photo deleted", func(t *testing.T) {
		e := setup(t)
		photo := attached(t, e, e.recipe.ID, adminOf(e))
		img, err := e.images.AddCommentImageToRecipe(ctx, e.recipe.ID, photo.ID, adminOf(e))
		if err != nil {
			t.Fatal(err)
		}
		if err := e.images.Delete(ctx, e.recipe.ID, img.ID, adminOf(e)); err != nil {
			t.Fatal(err)
		}
		for _, v := range []string{"thumb", "detail", "original"} {
			f, err := e.images.OpenCommentImage(ctx, e.recipe.ID, photo.ID, v)
			open(t, f, err)
		}
	})
}

func TestAddCommentImageFallsBackToCopy(t *testing.T) {
	e := setup(t)
	saved := *image.Link
	*image.Link = func(string, string) error { return errors.New("cross-device link") }
	t.Cleanup(func() { *image.Link = saved })

	photo := attached(t, e, e.recipe.ID, adminOf(e))
	img, err := e.images.AddCommentImageToRecipe(context.Background(), e.recipe.ID, photo.ID, adminOf(e))
	if err != nil {
		t.Fatal(err)
	}
	for i, src := range commentFiles(e, photo.ID) {
		dst := recipeFiles(e, img.ID)[i]
		a, errA := os.ReadFile(src)
		b, errB := os.ReadFile(dst)
		if errA != nil || errB != nil || !bytes.Equal(a, b) {
			t.Errorf("%s: copy differs (%v, %v)", dst, errA, errB)
		}
		sa, _ := os.Stat(src)
		sb, _ := os.Stat(dst)
		if os.SameFile(sa, sb) {
			t.Errorf("%s shares the inode although linking failed", dst)
		}
		if exists(dst + ".tmp") {
			t.Errorf("%s.tmp left behind", dst)
		}
	}
}

func TestSweepCommentImagesKeepsGoing(t *testing.T) {
	e := setup(t)
	now := time.Now()
	e.images.SetClock(func() time.Time { return now.Add(-25 * time.Hour) })
	a, _ := stage(t, e, adminOf(e))
	b, _ := stage(t, e, adminOf(e))
	saved := *image.RemoveAll
	failed := ""
	*image.RemoveAll = func(p string) error {
		if failed == "" {
			failed = p
			return errors.New("busy")
		}
		return saved(p)
	}
	t.Cleanup(func() { *image.RemoveAll = saved })

	if err := e.images.SweepCommentImages(context.Background(), now); err == nil {
		t.Fatal("err = nil, want the failed removal")
	}
	left := 0
	for _, id := range []string{a.ID, b.ID} {
		if exists(recipe.CommentImageDir(e.dir, e.recipe.ID, id)) {
			left++
		}
	}
	if left != 1 {
		t.Errorf("dirs left = %d, want 1 (only the failed one)", left)
	}
}

func TestCommentImageSweepLoopSweepsAtStart(t *testing.T) {
	e := setup(t)
	now := time.Now()
	e.images.SetClock(func() time.Time { return now.Add(-25 * time.Hour) })
	old, _ := stage(t, e, adminOf(e))
	e.images.SetClock(func() time.Time { return now })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.images.CommentImageSweepLoop(ctx, time.Hour, slog.New(slog.DiscardHandler)); close(done) }()
	defer func() { cancel(); <-done }()

	dir := recipe.CommentImageDir(e.dir, e.recipe.ID, old.ID)
	for deadline := time.Now().Add(5 * time.Second); exists(dir); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("old staged photo not swept at startup")
		}
	}
}

func TestAddCommentImageToRecipeHandler(t *testing.T) {
	h, tokens, samID, _ := newStack(t)
	cookie := loginCookie(t, h)
	r := createRecipe(t, h, cookie)
	body, ct := multipartBody(t, pngBytes(t, 64, 64), "image/png")
	rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/comment-images", body, ct, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("stage: %d %s", rec.Code, rec.Body.String())
	}
	var staged struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &staged); err != nil {
		t.Fatal(err)
	}
	comment, _ := json.Marshal(map[string]any{"body": "Lecker", "imageIds": []string{staged.ID}})
	if rec := doReq(h, http.MethodPost, "/api/v1/recipes/"+r.ID+"/comments", comment, "application/json", cookie); rec.Code != http.StatusCreated {
		t.Fatalf("comment: %d %s", rec.Code, rec.Body.String())
	}

	path := "/api/v1/recipes/" + r.ID + "/images/from-comment"
	req, _ := json.Marshal(map[string]string{"commentImageId": staged.ID})

	raw, _, err := tokens.Create(context.Background(), samID, "ro", []string{auth.ScopeRecipesRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	treq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(req))
	treq.Host = "localhost:8060"
	treq.Header.Set("Content-Type", "application/json")
	treq.Header.Set("Authorization", "Bearer "+raw)
	trec := httptest.NewRecorder()
	h.ServeHTTP(trec, treq)
	if trec.Code != http.StatusForbidden {
		t.Errorf("read token: %d %s", trec.Code, trec.Body.String())
	}

	rec = doReq(h, http.MethodPost, path, req, "application/json", cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add: %d %s", rec.Code, rec.Body.String())
	}
	var img recipe.Image
	if err := json.Unmarshal(rec.Body.Bytes(), &img); err != nil {
		t.Fatal(err)
	}
	if img.ID == "" || img.ID == staged.ID || img.Width != 64 {
		t.Errorf("image = %+v", img)
	}
	if rec := doReq(h, http.MethodPost, path, req, "application/json", cookie); rec.Code != http.StatusConflict {
		t.Errorf("second add: %d %s", rec.Code, rec.Body.String())
	}
}
