package recipe

import "path/filepath"

// MaxCommentImages is the most photos one comment carries.
const MaxCommentImages = 4

// CommentImage is a photo on a comment. InRecipe is true once it was added to
// the recipe's own photos.
type CommentImage struct {
	ID       string `json:"id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	InRecipe bool   `json:"inRecipe"`
}

// CommentImageDir is where one comment photo's variants live. It sits under
// the recipe's image directory, so deleting the recipe removes it too.
func CommentImageDir(base, recipeID, imageID string) string {
	return filepath.Join(base, recipeID, "comments", imageID)
}
