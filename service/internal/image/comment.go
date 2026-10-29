package image

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/user"
)

const (
	// maxStagedCommentImages caps the unsent photos one member holds; a new
	// one beyond it evicts the oldest.
	maxStagedCommentImages = 8
	// stagedCommentImageTTL is how long an unsent photo lives.
	stagedCommentImageTTL = 24 * time.Hour
)

// StageCommentImage renders the image in r into the recipe's comment photo
// directory and records it as staged for actor; a comment claims it later.
// Comments are allowed on locked recipes, so only the recipe's existence is
// checked, never the edit rule. When actor already holds
// maxStagedCommentImages unsent photos, the oldest one goes, so failed
// sends never lock anyone out. Errors: ErrNotFound, ErrUnsupported,
// ErrInvalid, ErrTooLarge.
func (s *Service) StageCommentImage(ctx context.Context, recipeID string, actor user.User, r io.Reader) (recipe.CommentImage, error) {
	if !IsID(recipeID) {
		return recipe.CommentImage{}, ErrNotFound
	}
	if _, err := s.q.GetRecipe(ctx, recipeID); errors.Is(err, sql.ErrNoRows) {
		return recipe.CommentImage{}, ErrNotFound
	} else if err != nil {
		return recipe.CommentImage{}, fmt.Errorf("get recipe: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return recipe.CommentImage{}, fmt.Errorf("read upload: %w", err)
	}
	if len(data) > MaxUploadBytes {
		return recipe.CommentImage{}, fmt.Errorf("%w: larger than %d bytes", ErrInvalid, MaxUploadBytes)
	}
	id := uuid.NewV7().String()
	dir := recipe.CommentImageDir(s.dir, recipeID, id)
	// ponytail: a crash between render and commit leaves dir without a row,
	// and the sweep only looks at rows; sweep row-less comments/<id> dirs
	// older than stagedCommentImageTTL if that ever adds up.
	w, err := s.render(ctx, data, dir, id)
	if err != nil {
		_ = os.RemoveAll(dir)
		return recipe.CommentImage{}, err
	}
	var evicted sqlc.DeleteOldestStagedCommentImageRow
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := q.GetRecipe(ctx, recipeID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("get recipe: %w", err)
		}
		n, err := q.CountStagedCommentImages(ctx, &actor.ID)
		if err != nil {
			return fmt.Errorf("count staged comment images: %w", err)
		}
		if n >= maxStagedCommentImages {
			if evicted, err = q.DeleteOldestStagedCommentImage(ctx, &actor.ID); err != nil {
				return fmt.Errorf("evict staged comment image: %w", err)
			}
		}
		if err := q.InsertCommentImage(ctx, sqlc.InsertCommentImageParams{
			ID: id, RecipeID: recipeID, UploaderID: &actor.ID,
			Width: int64(w.Width), Height: int64(w.Height), SizeBytes: w.Size,
			CreatedAt: db.FormatTime(s.now()),
		}); err != nil {
			return fmt.Errorf("insert comment image: %w", err)
		}
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return recipe.CommentImage{}, err
	}
	if evicted.ID != "" {
		// The row is gone already; a failed removal only wastes disk.
		if err := removeAll(recipe.CommentImageDir(s.dir, evicted.RecipeID, evicted.ID)); err != nil {
			slog.Warn("remove evicted comment image", "image", evicted.ID, "err", err)
		}
	}
	return recipe.CommentImage{ID: id, Width: w.Width, Height: w.Height}, nil
}

// OpenCommentImage opens one variant of a comment photo. Only photos that
// belong to a comment of recipeID are served; staged ones are private to
// their uploader's draft and answer ErrNotFound.
func (s *Service) OpenCommentImage(ctx context.Context, recipeID, imageID, variant string) (*os.File, error) {
	suffix, ok := variantSuffixes[variant]
	if !ok || !IsID(recipeID) || !IsID(imageID) {
		return nil, ErrNotFound
	}
	if _, err := s.q.GetAttachedCommentImage(ctx, sqlc.GetAttachedCommentImageParams{ID: imageID, RecipeID: recipeID}); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get comment image: %w", err)
	}
	f, err := os.Open(filepath.Join(recipe.CommentImageDir(s.dir, recipeID, imageID), imageID+suffix+".jpg"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open comment image: %w", err)
	}
	return f, nil
}

// SweepCommentImages deletes staged photos never sent within 24 hours, and
// those whose uploader is gone, with their files.
func (s *Service) SweepCommentImages(ctx context.Context, now time.Time) error {
	rows, err := s.q.SweepStagedCommentImages(ctx, db.FormatTime(now.Add(-stagedCommentImageTTL)))
	if err != nil {
		return fmt.Errorf("sweep staged comment images: %w", err)
	}
	// The rows are gone already, so one failed removal must not strand the rest.
	var errs []error
	for _, r := range rows {
		if err := removeAll(recipe.CommentImageDir(s.dir, r.RecipeID, r.ID)); err != nil {
			errs = append(errs, fmt.Errorf("remove comment image %s: %w", r.ID, err))
		}
	}
	return errors.Join(errs...)
}

// removeAll deletes a directory tree; a variable so tests can make it fail.
var removeAll = os.RemoveAll

// AddCommentImageToRecipe makes the comment photo commentImageID, attached
// to a comment of recipeID, an ordinary photo of the recipe under a new id:
// the next position, the cover when there is none. actor needs the same
// edit right as for an upload and is recorded as the recipe's editor.
// Errors: ErrNotFound (no attached photo of this recipe),
// recipe.ErrEditForbidden, ErrTooMany, ErrAlreadyInRecipe.
func (s *Service) AddCommentImageToRecipe(ctx context.Context, recipeID, commentImageID string, actor user.User) (recipe.Image, error) {
	if !IsID(recipeID) || !IsID(commentImageID) {
		return recipe.Image{}, ErrNotFound
	}
	// Checked before touching the disk, and again inside the transaction.
	if _, err := s.q.GetAttachedCommentImage(ctx, sqlc.GetAttachedCommentImageParams{ID: commentImageID, RecipeID: recipeID}); errors.Is(err, sql.ErrNoRows) {
		return recipe.Image{}, ErrNotFound
	} else if err != nil {
		return recipe.Image{}, fmt.Errorf("get comment image: %w", err)
	}
	id := uuid.NewV7().String()
	recipeDir := filepath.Join(s.dir, recipeID)
	srcDir := recipe.CommentImageDir(s.dir, recipeID, commentImageID)
	for _, v := range variants {
		if err := copyFile(filepath.Join(srcDir, commentImageID+v.suffix+".jpg"), variantPath(s.dir, recipeID, id, v.suffix)); err != nil {
			removeFiles(recipeDir, id)
			if errors.Is(err, os.ErrNotExist) {
				return recipe.Image{}, ErrNotFound // the comment went meanwhile
			}
			return recipe.Image{}, err
		}
	}

	var out recipe.Image
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := guard(ctx, q, actor, recipeID)
		if err != nil {
			return err
		}
		photo, err := q.GetAttachedCommentImage(ctx, sqlc.GetAttachedCommentImageParams{ID: commentImageID, RecipeID: recipeID})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return fmt.Errorf("get comment image: %w", err)
		}
		// Deleting the recipe photo clears recipe_image_id (ON DELETE SET NULL).
		if photo.RecipeImageID != nil {
			return ErrAlreadyInRecipe
		}
		if out, err = s.insertRecipeImage(ctx, q, row, actor.ID, id, photo.Width, photo.Height, photo.SizeBytes); err != nil {
			return err
		}
		if err := q.SetCommentImageRecipeImage(ctx, sqlc.SetCommentImageRecipeImageParams{RecipeImageID: &id, ID: commentImageID}); err != nil {
			return fmt.Errorf("set recipe image: %w", err)
		}
		return nil
	})
	if err != nil {
		removeFiles(recipeDir, id)
		return recipe.Image{}, err
	}
	return out, nil
}

// copyFile copies src to dst through a ".tmp" sibling renamed into place,
// so a reader never sees a partial file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	return nil
}

type commentImageOutput struct {
	Body recipe.CommentImage
}

// FromCommentInput is the body of the add-comment-image-to-recipe operation.
type FromCommentInput struct {
	CommentImageID string `json:"commentImageId" minLength:"1" doc:"A photo attached to a comment of this recipe"`
}

type fromCommentInput struct {
	ID   string `path:"id"`
	Body FromCommentInput
}

type commentFileInput struct {
	RecipeID string `path:"recipeId"`
	ImageID  string `path:"imageId"`
	File     string `path:"file" doc:"thumb.jpg, detail.jpg or original.jpg; anything else is a 404"`
}

func registerCommentImages(api huma.API, svc *Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "upload-comment-image",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/comment-images",
		Summary:       "Stage a photo for a comment (JPEG, PNG or WebP, at most 10 MiB)",
		Tags:          []string{"comments"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusCreated,
		MaxBodyBytes:  MaxUploadBytes,
		Middlewares:   huma.Middlewares{LimitUpload(api)},
		Errors:        []int{403, 404, 413, 415, 422},
	}, func(ctx context.Context, in *uploadInput) (*commentImageOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		file := in.RawBody.Data().File
		defer file.Close()
		img, err := svc.StageCommentImage(ctx, in.ID, u, file)
		if err != nil {
			return nil, imageErr(err)
		}
		return &commentImageOutput{Body: img}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "add-comment-image-to-recipe",
		Method:        http.MethodPost,
		Path:          "/api/v1/recipes/{id}/images/from-comment",
		Summary:       "Add a comment photo to the recipe photos",
		Tags:          []string{"images"},
		Security:      auth.Protected(auth.ScopeRecipesWrite),
		DefaultStatus: http.StatusCreated,
		Errors:        []int{403, 404, 409, 422},
	}, func(ctx context.Context, in *fromCommentInput) (*imageOutput, error) {
		u, ok := auth.UserFrom(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		img, err := svc.AddCommentImageToRecipe(ctx, in.ID, in.Body.CommentImageID, u)
		if err != nil {
			return nil, imageErr(err)
		}
		return &imageOutput{Body: img}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-comment-image-file",
		Method:      http.MethodGet,
		Path:        "/images/{recipeId}/comments/{imageId}/{file}",
		Summary:     "Read a comment photo in one of its rendered sizes",
		Tags:        []string{"comments"},
		Security:    auth.Protected(auth.ScopeRecipesRead),
		Errors:      []int{403, 404},
		Responses:   JPEGResponses,
	}, func(ctx context.Context, in *commentFileInput) (*huma.StreamResponse, error) {
		variant, ok := strings.CutSuffix(in.File, ".jpg")
		if !ok {
			return nil, huma.Error404NotFound("image not found")
		}
		f, err := svc.OpenCommentImage(ctx, in.RecipeID, in.ImageID, variant)
		if errors.Is(err, ErrNotFound) {
			return nil, huma.Error404NotFound("image not found")
		}
		if err != nil {
			return nil, err
		}
		return JPEG(f, immutableCache), nil
	})
}
