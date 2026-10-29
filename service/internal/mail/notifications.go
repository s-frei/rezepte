package mail

import (
	"errors"
	"fmt"
	htmltemplate "html/template"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/s-frei/rezepte/service/internal/user"
)

// CommentReason says why a follower gets a comment mail.
type CommentReason string

// The values of CommentReason: the recipient created the recipe, or wrote
// a comment on it before.
const (
	ReasonCreated   CommentReason = "created"
	ReasonCommented CommentReason = "commented"
)

// Comment tells a follower of a recipe about a new comment. RecipeURLPath
// is appended to REZEPTE_PUBLIC_URL; Body is plain text.
type Comment struct {
	To, RecipeTitle, RecipeURLPath, AuthorName string
	AuthorColor                                user.Color
	// AuthorAvatar is the author's picture as JPEG; nil shows the initial.
	AuthorAvatar []byte
	Body         string
	// Photos are the comment's photos as JPEG (image.MailPhoto), in order;
	// each is an inline part under the quote.
	Photos      [][]byte
	At          time.Time
	Reason      CommentReason
	Unsubscribe Unsubscribe
	Locale      user.Locale
}

// userColors are the light values of --color-user-<color> and
// --color-user-<color>-foreground in frontend/src/app.css: a mail cannot
// read the stylesheet, so the author's circle inlines them.
var userColors = map[user.Color]struct{ bg, fg string }{
	"amber": {"#f0dfc4", "#7c4a08"},
	"clay":  {"#f3dbcd", "#8a4527"},
	"rose":  {"#f5d9da", "#8c3b46"},
	"plum":  {"#e9dcea", "#6f3f6e"},
	"sage":  {"#dbe7d9", "#3f6b48"},
	"olive": {"#e6e3cf", "#5f6330"},
	"teal":  {"#d5e5e4", "#2f6260"},
	"slate": {"#dedfe4", "#4a5263"},
}

// circle is what a person's circle in a mail shows without a picture: the
// light colors of their palette color, amber for an unknown one, and the
// initial of their name, or "?" for an empty one.
func circle(color user.Color, name string) (bg, fg, initial string) {
	colors, ok := userColors[color]
	if !ok {
		colors = userColors["amber"]
	}
	r, _ := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError {
		return colors.bg, colors.fg, "?"
	}
	return colors.bg, colors.fg, strings.ToUpper(string(r))
}

// BuildComment renders the comment mail; its button opens the recipe on
// the comments tab.
func BuildComment(cfg Config, in Comment, publicURL string, now time.Time) (Envelope, error) {
	bg, fg, initial := circle(in.AuthorColor, in.AuthorName)
	// The picture's Content-ID shares the Message-ID's random part: RFC 2392
	// wants it unique, and a client that caches by cid must not show one
	// author's picture in another's mail.
	id := newID()
	var pictures []inline
	avatar := ""
	if len(in.AuthorAvatar) > 0 {
		cid := "avatar." + id + "@" + domainOf(cfg.From)
		avatar = "cid:" + cid
		pictures = []inline{{CID: cid, JPEG: in.AuthorAvatar}}
	}
	domain := domainOf(cfg.From)
	var photos []htmltemplate.URL
	for i, p := range in.Photos {
		cid := fmt.Sprintf("photo.%d.%s@%s", i, id, domain)
		pictures = append(pictures, inline{CID: cid, JPEG: p})
		photos = append(photos, htmltemplate.URL("cid:"+cid)) //nolint:gosec // G203: a cid: URL built from an index, hex and the sender domain.
	}
	more := ""
	if n := len(in.Photos); n > 0 {
		key := "mail_comment_photos_other"
		if n == 1 {
			key = "mail_comment_photos_one"
		}
		var err error
		if more, err = text(in.Locale, key, map[string]string{"count": strconv.Itoa(n)}); err != nil {
			return Envelope{}, err
		}
	}
	return build(cfg, in.To, letter{
		Name:        "comment",
		Lang:        string(in.Locale),
		SubjectKey:  "mail_comment_subject",
		SubjectArgs: map[string]string{"author": in.AuthorName, "title": in.RecipeTitle},
		ID:          id,
		PublicURL:   publicURL,
		Unsubscribe: in.Unsubscribe,
		Inline:      pictures,
		Data: struct {
			Title, Author, Initial, Bg, Fg, Body, Reason, URL string
			More                                              string
			Avatar                                            htmltemplate.URL
			Photos                                            []htmltemplate.URL
			At                                                time.Time
		}{in.RecipeTitle, in.AuthorName, initial, bg, fg, in.Body,
			string(in.Reason), publicURL + in.RecipeURLPath, more, htmltemplate.URL(avatar), photos, in.At}, //nolint:gosec // G203: avatar is empty or a cid: URL built above from hex and the sender domain.
	}, now)
}

// Tasty tells a recipe's author that somebody marked it tasty.
// RecipeURLPath is appended to REZEPTE_PUBLIC_URL.
type Tasty struct {
	To, RecipeTitle, RecipeURLPath, MarkerName string
	MarkerColor                                user.Color
	// MarkerAvatar is the marker's picture as JPEG; nil shows the initial.
	MarkerAvatar []byte
	Unsubscribe  Unsubscribe
	Locale       user.Locale
}

// BuildTasty renders the tasty mail; its button opens the recipe.
func BuildTasty(cfg Config, in Tasty, publicURL string, now time.Time) (Envelope, error) {
	bg, fg, initial := circle(in.MarkerColor, in.MarkerName)
	// The picture's Content-ID shares the Message-ID's random part, as in
	// BuildComment.
	id := newID()
	var pictures []inline
	avatar := ""
	if len(in.MarkerAvatar) > 0 {
		cid := "avatar." + id + "@" + domainOf(cfg.From)
		avatar = "cid:" + cid
		pictures = []inline{{CID: cid, JPEG: in.MarkerAvatar}}
	}
	return build(cfg, in.To, letter{
		Name:        "tasty",
		Lang:        string(in.Locale),
		SubjectKey:  "mail_tasty_subject",
		SubjectArgs: map[string]string{"name": in.MarkerName, "title": in.RecipeTitle},
		ID:          id,
		PublicURL:   publicURL,
		Unsubscribe: in.Unsubscribe,
		Inline:      pictures,
		Data: struct {
			Title, Name, Initial, Bg, Fg, URL string
			Avatar                            htmltemplate.URL
		}{in.RecipeTitle, in.MarkerName, initial, bg, fg, publicURL + in.RecipeURLPath, htmltemplate.URL(avatar)}, //nolint:gosec // G203: avatar is empty or a cid: URL built above from hex and the sender domain.
	}, now)
}

// InviteAccepted tells the admin who issued a setup link that the new
// person set up their account.
type InviteAccepted struct {
	To, Username, DisplayName string
	Unsubscribe               Unsubscribe
	Locale                    user.Locale
}

// BuildInviteAccepted renders the invitation-accepted mail; its button
// opens the people settings.
func BuildInviteAccepted(cfg Config, in InviteAccepted, publicURL string, now time.Time) (Envelope, error) {
	return build(cfg, in.To, letter{
		Name:        "invite_accepted",
		Lang:        string(in.Locale),
		SubjectKey:  "mail_invite_accepted_subject",
		SubjectArgs: map[string]string{"name": in.DisplayName},
		PublicURL:   publicURL,
		Unsubscribe: in.Unsubscribe,
		Data: struct{ Name, Username, URL string }{
			in.DisplayName, in.Username, publicURL + "/settings/users"},
	}, now)
}

// DigestRecipe is one recipe in the new recipes digest, as it is at send
// time. URLPath ("/recipes/<slug>") is appended to REZEPTE_PUBLIC_URL.
type DigestRecipe struct {
	// CreatorName is empty when the creator's account is gone; the mail
	// then names a former member.
	Title, URLPath, CreatorName string
	CreatorColor                user.Color
	// CreatorAvatar is the creator's picture as JPEG; nil shows the initial.
	CreatorAvatar []byte
	// Photo is the cover as JPEG (image.MailPhoto); nil is a card without.
	Photo    []byte
	Servings int
	// Minutes is prep plus cook time; 0 is unknown and not shown.
	Minutes int
	// CreatedAt is when the recipe was added.
	CreatedAt time.Time
}

// RecipeDigest tells one person about the recipes others created since the
// last digest, in creation order; Recipes is never empty.
type RecipeDigest struct {
	To          string
	Recipes     []DigestRecipe
	Unsubscribe Unsubscribe
	Locale      user.Locale
}

// The digest shows the first digestCards recipes as cards with photo and
// button, the next digestRows as a plain list, and counts the rest, so a
// script creating 80 recipes still yields a small mail.
const (
	digestCards = 6
	digestRows  = 30
)

type digestCard struct {
	Title, URL, Creator, Initial, Bg, Fg, Facts string
	Photo, Avatar                               htmltemplate.URL
	CreatedAt                                   time.Time
}

type digestRow struct{ Title, URL, Creator string }

// BuildRecipeDigest renders the daily new recipes mail. Each photo and each
// distinct creator picture of the cards is one inline part.
func BuildRecipeDigest(cfg Config, in RecipeDigest, publicURL string, now time.Time) (Envelope, error) {
	if len(in.Recipes) == 0 {
		return Envelope{}, errors.New("recipe digest without recipes")
	}
	former, err := text(in.Locale, "mail_recipe_digest_former_member", nil)
	if err != nil {
		return Envelope{}, err
	}
	id := newID()
	domain := domainOf(cfg.From)
	var pictures []inline
	avatars := map[string]string{} // JPEG bytes to its cid: URL
	var cards []digestCard
	var rows []digestRow
	for i, r := range in.Recipes {
		if r.CreatorName == "" {
			r.CreatorName = former
		}
		url := publicURL + r.URLPath
		if i >= digestCards {
			if i < digestCards+digestRows {
				rows = append(rows, digestRow{r.Title, url, r.CreatorName})
			}
			continue
		}
		bg, fg, initial := circle(r.CreatorColor, r.CreatorName)
		facts, err := digestFacts(in.Locale, r)
		if err != nil {
			return Envelope{}, err
		}
		c := digestCard{Title: r.Title, URL: url, Creator: r.CreatorName, Initial: initial, Bg: bg, Fg: fg, Facts: facts, CreatedAt: r.CreatedAt}
		// The Content-IDs share the Message-ID's random part, as in
		// BuildComment.
		if len(r.Photo) > 0 {
			cid := fmt.Sprintf("photo.%d.%s@%s", i, id, domain)
			pictures = append(pictures, inline{CID: cid, JPEG: r.Photo})
			c.Photo = htmltemplate.URL("cid:" + cid) //nolint:gosec // G203: a cid: URL built from an index, hex and the sender domain.
		}
		if len(r.CreatorAvatar) > 0 {
			url, ok := avatars[string(r.CreatorAvatar)]
			if !ok {
				cid := fmt.Sprintf("avatar.%d.%s@%s", len(avatars), id, domain)
				pictures = append(pictures, inline{CID: cid, JPEG: r.CreatorAvatar})
				url = "cid:" + cid
				avatars[string(r.CreatorAvatar)] = url
			}
			c.Avatar = htmltemplate.URL(url) //nolint:gosec // G203: as Photo above.
		}
		cards = append(cards, c)
	}
	subjectKey, subjectArgs := "mail_recipe_digest_subject_many", map[string]string{"count": strconv.Itoa(len(in.Recipes))}
	if len(in.Recipes) == 1 {
		subjectKey, subjectArgs = "mail_recipe_digest_subject_one", map[string]string{"title": in.Recipes[0].Title}
	}
	countKey := "mail_recipe_digest_count_other"
	if len(in.Recipes) == 1 {
		countKey = "mail_recipe_digest_count_one"
	}
	count, err := text(in.Locale, countKey, map[string]string{"count": strconv.Itoa(len(in.Recipes))})
	if err != nil {
		return Envelope{}, err
	}
	more := ""
	if n := len(in.Recipes) - digestCards - digestRows; n > 0 {
		more = strconv.Itoa(n)
	}
	return build(cfg, in.To, letter{
		Name:        "recipe_digest",
		Lang:        string(in.Locale),
		SubjectKey:  subjectKey,
		SubjectArgs: subjectArgs,
		ID:          id,
		PublicURL:   publicURL,
		Unsubscribe: in.Unsubscribe,
		Inline:      pictures,
		Data: struct {
			Cards               []digestCard
			Rows                []digestRow
			Count, More, AllURL string
		}{cards, rows, count, more, publicURL + "/recipes"},
	}, now)
}

// digestFacts is a card's line of short facts: the servings and, when
// known, the time.
func digestFacts(locale user.Locale, r DigestRecipe) (string, error) {
	key := "mail_recipe_digest_servings_other"
	if r.Servings == 1 {
		key = "mail_recipe_digest_servings_one"
	}
	facts, err := text(locale, key, map[string]string{"count": strconv.Itoa(r.Servings)})
	if err != nil || r.Minutes <= 0 {
		return facts, err
	}
	minutes, err := text(locale, "mail_recipe_digest_minutes", map[string]string{"count": strconv.Itoa(r.Minutes)})
	return facts + " · " + minutes, err
}
