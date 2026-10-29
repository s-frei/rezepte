package image

// Link lets the external tests make hard links fail and force the copy
// fallback of AddCommentImageToRecipe.
var Link = &link

// RemoveAll lets the external tests make the sweep's file removal fail.
var RemoveAll = &removeAll
