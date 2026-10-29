package recipe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
	"github.com/s-frei/rezepte/service/internal/user"
)

// MaxCommentRunes is the longest comment, counted in runes after
// trimming: about half a printed page.
const MaxCommentRunes = 2000

// Errors of recipe comments; the HTTP layer maps them to statuses.
var (
	ErrCommentNotFound        = errors.New("comment not found")
	ErrNotCommentAuthor       = errors.New("only the person who wrote a comment may edit it")
	ErrCommentDeleteForbidden = errors.New("only the person who wrote a comment or an admin may delete it")
	ErrInvalidComment         = errors.New("a comment is 1 to 2000 characters of plain text")
	ErrInvalidCommentImages   = errors.New("a comment carries up to 4 different photos the writer uploaded for this recipe")
)

// Comment is one comment on a recipe, as one viewer sees it.
// Author is nil for a former member. New, CanEdit and CanDelete are for the
// viewer the entry was loaded for.
type Comment struct {
	ID        int64          `json:"id"`
	Author    *Person        `json:"author,omitempty" doc:"Who wrote the comment; absent once their account was removed"`
	Body      string         `json:"body"`
	CreatedAt time.Time      `json:"createdAt"`
	EditedAt  *time.Time     `json:"editedAt" nullable:"true"`
	New       bool           `json:"new" doc:"Written by somebody else after the viewer last opened the recipe's comments; only ever true for the recipe's author and members with a comment on it"`
	CanEdit   bool           `json:"canEdit"`
	CanDelete bool           `json:"canDelete"`
	Images    []CommentImage `json:"images" doc:"Photos in upload order; empty when there are none"`
}

// NormalizeCommentBody trims s, turns \r\n and \r into \n, and refuses an
// empty result, more than MaxCommentRunes runes, control characters other
// than newline and tab, and invisible format characters (zero-width space,
// BOM, soft hyphen, bidi overrides). Zero-width joiner and non-joiner stay:
// emoji sequences and some scripts need them, but they alone are not
// content, so the body needs one other visible rune.
func NormalizeCommentBody(s string) (string, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > MaxCommentRunes {
		return "", ErrInvalidComment
	}
	visible := false
	for _, r := range s {
		joiner := r == '\u200C' || r == '\u200D'
		if unicode.IsControl(r) && r != '\n' && r != '\t' || unicode.Is(unicode.Cf, r) && !joiner {
			return "", ErrInvalidComment
		}
		visible = visible || !joiner && !unicode.IsSpace(r)
	}
	if !visible {
		return "", ErrInvalidComment
	}
	return s, nil
}

// requireRecipe loads recipeID through q, or returns ErrNotFound. Writers
// pass their transaction's q, so the check and the write are atomic.
func requireRecipe(ctx context.Context, q *sqlc.Queries, recipeID string) (sqlc.Recipe, error) {
	r, err := q.GetRecipe(ctx, recipeID)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.Recipe{}, ErrNotFound
	}
	if err != nil {
		return sqlc.Recipe{}, fmt.Errorf("get recipe %s: %w", recipeID, err)
	}
	return r, nil
}

// watermark is the highest comment id userID has seen on recipeID; 0 when
// they never opened it.
func (s *Service) watermark(ctx context.Context, userID, recipeID string) (int64, error) {
	seen, err := s.q.GetCommentWatermark(ctx, sqlc.GetCommentWatermarkParams{UserID: userID, RecipeID: recipeID})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get comment watermark: %w", err)
	}
	return seen, nil
}

// Comments returns recipeID's entries oldest first, as viewer sees them.
// It does not mark anything seen: New reflects the state before this call,
// and only for the audience of the card dot: the recipe's author and people
// who wrote an entry on it. Everyone else never sees an entry as new.
func (s *Service) Comments(ctx context.Context, viewer user.User, recipeID string) ([]Comment, error) {
	r, err := requireRecipe(ctx, s.q, recipeID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListComments(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	seen, err := s.watermark(ctx, viewer.ID, recipeID)
	if err != nil {
		return nil, err
	}
	people, err := s.commentAuthors(ctx, rows)
	if err != nil {
		return nil, err
	}
	photos, err := s.q.ListCommentImagesForRecipe(ctx, recipeID)
	if err != nil {
		return nil, fmt.Errorf("list comment images: %w", err)
	}
	byComment := make(map[int64][]sqlc.CommentImage)
	for _, p := range photos {
		byComment[*p.CommentID] = append(byComment[*p.CommentID], p)
	}
	follows := r.CreatedBy == viewer.ID
	for _, row := range rows {
		follows = follows || (row.AuthorID != nil && *row.AuthorID == viewer.ID)
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		c, err := toComment(row, people, byComment[row.ID], viewer)
		if err != nil {
			return nil, err
		}
		c.New = follows && !c.mine(viewer) && row.ID > seen
		out = append(out, c)
	}
	return out, nil
}

// AddComment stores body as viewer's entry on recipeID with the photos
// imageIDs, which viewer staged on recipeID, and, once committed, tells the
// CommentNotifier. It leaves viewer's watermark alone: only opening the
// comments clears what is new, so an entry somebody else wrote meanwhile
// stays new until it was shown.
func (s *Service) AddComment(ctx context.Context, viewer user.User, recipeID, body string, imageIDs []string) (Comment, error) {
	body, err := NormalizeCommentBody(body)
	if err != nil {
		return Comment{}, err
	}
	if err := checkCommentImageIDs(imageIDs); err != nil {
		return Comment{}, err
	}
	var id int64
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := requireRecipe(ctx, q, recipeID); err != nil {
			return err
		}
		authorID := viewer.ID
		var err error
		id, err = q.InsertComment(ctx, sqlc.InsertCommentParams{
			RecipeID: recipeID, AuthorID: &authorID, Body: body, CreatedAt: db.FormatTime(s.now()),
		})
		if err != nil {
			return fmt.Errorf("insert comment: %w", err)
		}
		return attachCommentImages(ctx, q, id, recipeID, viewer.ID, imageIDs)
	})
	if err != nil {
		return Comment{}, err
	}
	if s.notifier != nil {
		s.notifier.CommentAdded(ctx, recipeID, id)
	}
	return s.comment(ctx, viewer, id)
}

// EditComment replaces the body of viewer's own entry id. A non-nil
// imageIDs is the full set of photos it keeps: ones not in it are removed,
// new ones must be staged by viewer on the recipe; nil leaves the photos
// alone. edited_at moves only when the body or the photo set changed, so a
// no-op edit returns the entry as it was. The id stays, so an edit never
// makes the entry new again for anyone.
func (s *Service) EditComment(ctx context.Context, viewer user.User, id int64, body string, imageIDs *[]string) (Comment, error) {
	body, err := NormalizeCommentBody(body)
	if err != nil {
		return Comment{}, err
	}
	if imageIDs != nil {
		if err := checkCommentImageIDs(*imageIDs); err != nil {
			return Comment{}, err
		}
	}
	var recipeID string
	var removed []string
	err = db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getComment(ctx, q, id)
		if err != nil {
			return err
		}
		if row.AuthorID == nil || *row.AuthorID != viewer.ID {
			return ErrNotCommentAuthor
		}
		recipeID = row.RecipeID
		photosChanged := false
		if imageIDs != nil {
			current, err := q.ListCommentImageIDs(ctx, &id)
			if err != nil {
				return fmt.Errorf("list comment images: %w", err)
			}
			photosChanged = !slices.Equal(slices.Sorted(slices.Values(current)), slices.Sorted(slices.Values(*imageIDs)))
		}
		if photosChanged {
			keep, err := json.Marshal(*imageIDs)
			if err != nil {
				return fmt.Errorf("marshal image ids: %w", err)
			}
			if removed, err = q.DeleteCommentImages(ctx, sqlc.DeleteCommentImagesParams{CommentID: &id, Keep: string(keep)}); err != nil {
				return fmt.Errorf("delete comment images: %w", err)
			}
			if err := attachCommentImages(ctx, q, id, row.RecipeID, viewer.ID, *imageIDs); err != nil {
				return err
			}
		}
		if body == row.Body && !photosChanged {
			return nil
		}
		edited := db.FormatTime(s.now())
		if err := q.UpdateCommentBody(ctx, sqlc.UpdateCommentBodyParams{Body: body, EditedAt: &edited, ID: id}); err != nil {
			return fmt.Errorf("update comment: %w", err)
		}
		return nil
	})
	if err != nil {
		return Comment{}, err
	}
	s.removeCommentImageDirs(recipeID, removed)
	return s.comment(ctx, viewer, id)
}

// DeleteComment removes entry id when viewer wrote it or is an admin, and
// with it its photos.
func (s *Service) DeleteComment(ctx context.Context, viewer user.User, id int64) error {
	var recipeID string
	var removed []string
	err := db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		row, err := getComment(ctx, q, id)
		if err != nil {
			return err
		}
		own := row.AuthorID != nil && *row.AuthorID == viewer.ID
		if !own && !viewer.Role.IsAdmin() {
			return ErrCommentDeleteForbidden
		}
		recipeID = row.RecipeID
		if removed, err = q.DeleteCommentImages(ctx, sqlc.DeleteCommentImagesParams{CommentID: &id, Keep: "[]"}); err != nil {
			return fmt.Errorf("delete comment images: %w", err)
		}
		if err := q.DeleteComment(ctx, id); err != nil {
			return fmt.Errorf("delete comment: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.removeCommentImageDirs(recipeID, removed)
	return nil
}

// checkCommentImageIDs refuses more than MaxCommentImages ids and duplicates.
func checkCommentImageIDs(ids []string) error {
	if len(ids) > MaxCommentImages || len(slices.Compact(slices.Sorted(slices.Values(ids)))) != len(ids) {
		return ErrInvalidCommentImages
	}
	return nil
}

// attachCommentImages hands the photos ids, which uploaderID staged on
// recipeID or which already belong to commentID, to comment commentID. Any
// id that is not such a photo refuses all of them.
func attachCommentImages(ctx context.Context, q *sqlc.Queries, commentID int64, recipeID, uploaderID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("marshal image ids: %w", err)
	}
	n, err := q.AttachCommentImages(ctx, sqlc.AttachCommentImagesParams{
		CommentID: &commentID, Ids: string(idsJSON), RecipeID: recipeID, UploaderID: &uploaderID,
	})
	if err != nil {
		return fmt.Errorf("attach comment images: %w", err)
	}
	if n != int64(len(ids)) {
		return ErrInvalidCommentImages
	}
	return nil
}

// removeCommentImageDirs deletes the files of photos whose rows are gone.
// Best effort, like Delete: a leftover directory is an orphan, not an
// inconsistency.
func (s *Service) removeCommentImageDirs(recipeID string, ids []string) {
	if s.imageDir == "" {
		return
	}
	for _, id := range ids {
		if err := os.RemoveAll(CommentImageDir(s.imageDir, recipeID, id)); err != nil {
			slog.Warn("remove comment image dir", "recipe", recipeID, "image", id, "err", err)
		}
	}
}

// MarkCommentsSeen raises viewerID's watermark on recipeID to upTo, the
// highest entry id the page showed, capped at the recipe's newest entry so a
// stale or forged value cannot pre-mark entries not written yet. The
// watermark never lowers; upTo 0 or an empty diary writes nothing.
func (s *Service) MarkCommentsSeen(ctx context.Context, viewerID, recipeID string, upTo int64) error {
	return db.Tx(ctx, s.conn, func(q *sqlc.Queries) error {
		if _, err := requireRecipe(ctx, q, recipeID); err != nil {
			return err
		}
		top, err := q.MaxCommentID(ctx, recipeID)
		if err != nil {
			return fmt.Errorf("max comment id: %w", err)
		}
		seen := min(upTo, top)
		if seen <= 0 {
			return nil
		}
		if err := q.RaiseCommentWatermark(ctx, sqlc.RaiseCommentWatermarkParams{
			UserID: viewerID, RecipeID: recipeID, LastSeen: seen,
		}); err != nil {
			return fmt.Errorf("raise comment watermark: %w", err)
		}
		return nil
	})
}

// newCommentsFor is the card batch: which of ids show the dot for userID.
func (s *Service) newCommentsFor(ctx context.Context, userID string, ids []string) (map[string]bool, error) {
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return nil, fmt.Errorf("marshal recipe ids: %w", err)
	}
	rows, err := s.q.ListRecipesWithNewComments(ctx, sqlc.ListRecipesWithNewCommentsParams{
		RecipeIds: string(idsJSON), UserID: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("list recipes with new comments: %w", err)
	}
	out := make(map[string]bool, len(rows))
	for _, id := range rows {
		out[id] = true
	}
	return out, nil
}

func getComment(ctx context.Context, q *sqlc.Queries, id int64) (sqlc.RecipeComment, error) {
	row, err := q.GetComment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.RecipeComment{}, ErrCommentNotFound
	}
	if err != nil {
		return sqlc.RecipeComment{}, fmt.Errorf("get comment %d: %w", id, err)
	}
	return row, nil
}

// comment loads one entry as viewer sees it right after viewer changed it,
// so it is never new.
func (s *Service) comment(ctx context.Context, viewer user.User, id int64) (Comment, error) {
	row, err := getComment(ctx, s.q, id)
	if err != nil {
		return Comment{}, err
	}
	people, err := s.commentAuthors(ctx, []sqlc.RecipeComment{row})
	if err != nil {
		return Comment{}, err
	}
	photos, err := s.q.ListCommentImagesForComment(ctx, &id)
	if err != nil {
		return Comment{}, fmt.Errorf("list comment images: %w", err)
	}
	return toComment(row, people, photos, viewer)
}

// commentAuthors loads the people behind rows in one query.
func (s *Service) commentAuthors(ctx context.Context, rows []sqlc.RecipeComment) (map[string]Person, error) {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.AuthorID != nil {
			ids = append(ids, *r.AuthorID)
		}
	}
	return s.peopleByID(ctx, ids)
}

// peopleByID loads the people behind ids, duplicates allowed, in one query.
// An id without an account is missing from the map.
func (s *Service) peopleByID(ctx context.Context, ids []string) (map[string]Person, error) {
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	people := make(map[string]Person, len(ids))
	if len(ids) == 0 {
		return people, nil
	}
	rows, err := s.q.ListAuthorsForIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list authors: %w", err)
	}
	for _, a := range rows {
		people[a.ID] = Person{ID: a.ID, Username: a.Username, DisplayName: a.DisplayName, Color: a.Color, AvatarID: a.AvatarID}
	}
	return people, nil
}

func toComment(row sqlc.RecipeComment, people map[string]Person, photos []sqlc.CommentImage, viewer user.User) (Comment, error) {
	created, err := db.ParseTime(row.CreatedAt)
	if err != nil {
		return Comment{}, err
	}
	c := Comment{ID: row.ID, Body: row.Body, CreatedAt: created, Images: make([]CommentImage, 0, len(photos))}
	for _, p := range photos {
		c.Images = append(c.Images, CommentImage{ID: p.ID, Width: int(p.Width), Height: int(p.Height), InRecipe: p.RecipeImageID != nil})
	}
	if row.EditedAt != nil {
		edited, err := db.ParseTime(*row.EditedAt)
		if err != nil {
			return Comment{}, err
		}
		c.EditedAt = &edited
	}
	if row.AuthorID != nil {
		if p, ok := people[*row.AuthorID]; ok {
			c.Author = &p
		}
	}
	c.CanEdit = c.mine(viewer)
	c.CanDelete = c.CanEdit || viewer.Role.IsAdmin()
	return c, nil
}

func (c Comment) mine(viewer user.User) bool {
	return c.Author != nil && c.Author.ID == viewer.ID
}
