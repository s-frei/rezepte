package image

import "time"

// Link lets the external tests make hard links fail and force the copy
// fallback of AddCommentImageToRecipe.
var Link = &link

// RemoveAll lets the external tests make the sweep's file removal fail.
var RemoveAll = &removeAll

// SetLinkTTL lets the external tests issue links that are already expired.
func SetLinkTTL(l *UploadLinks, ttl time.Duration) { l.ttl = ttl }
