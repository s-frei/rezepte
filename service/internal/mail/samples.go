package mail

import (
	"time"

	"github.com/s-frei/rezepte/service/internal/user"
)

// Sample is one rendering of a template that mail-preview shows and
// TestSamplesRender builds. Variant tells apart samples of one template.
// Section is the kind of mail the preview files it under, Audience says who
// gets it, Roles which roles that can be, and OptOut the unsubscribe group
// that switches it off, empty when nothing can.
type Sample struct {
	Template, Variant string
	Locale            user.Locale
	Section           string
	Audience, OptOut  string
	Roles             []user.Role
	Build             func(cfg Config, now time.Time) (Envelope, error)
}

// The roles a mail can reach, for audiences.
var (
	toAnyone   = []user.Role{user.RoleSuperadmin, user.RoleAdmin, user.RoleUser}
	toNotOwner = []user.Role{user.RoleAdmin, user.RoleUser}
	toAdmins   = []user.Role{user.RoleSuperadmin, user.RoleAdmin}
	toOwner    = []user.Role{user.RoleSuperadmin}
)

// audiences describes each template's recipients for mail-preview. The
// rules live where the mails are sent; this restates them for a reader.
var audiences = map[string]struct {
	section, who, optOut string
	roles                []user.Role
}{
	"invite": {"Account", "A person an admin or the owner sends a setup link, at the account's email address whether confirmed or not: someone invited, " +
		"or an existing account, which gets its own copy. Not when the admin only shows the link.", "", toNotOwner},
	"reset": {"Account", "Admins and members with a password and a confirmed email address, on their own request: at most once in five minutes, " +
		"and not while an admin's setup link is open. Never the owner.", "", toNotOwner},
	"hint": {"Account", "Admins and members without a password, with an identity at the configured provider and a confirmed email address, " +
		"on their own request. Never the owner.", "", toNotOwner},
	"confirm": {"Account", "Any account, at a new email address it has to confirm: entered in the profile (at most once a minute), by an admin, " +
		"or with a new account that has a password. Not when a setup link mailed there confirms it.", "", toAnyone},
	"password_changed": {"Security notices", "Any account with a confirmed email address (owner, admin or member) when its password is set or changed. " +
		"The owner's has no button.", "", toAnyone},
	"email_changed": {"Security notices", "Any account, at its old email address if that was confirmed, when the address is changed or removed.", "", toAnyone},
	"identity_linked": {"Security notices", "Any account with a confirmed email address that could sign in already, when a sign-in identity is linked " +
		"in the profile or through a setup link.", "", toAnyone},
	"comment": {"Notifications", "Every role, at a confirmed email address, unless switched off: whoever owns the recipe (created or inherited it) " +
		"or commented on it before, never the author.", "household", toAnyone},
	"recipe_digest": {"Notifications", "Every role at a confirmed email address, unless switched off, once a day at the owner's hour: " +
		"the recipes other people created since the last run, at least an hour old. Nobody whose only new recipes are their own.", "household", toAnyone},
	"invite_accepted": {"Notifications", "The admin or owner who issued a setup link, when a new person (an account that could not sign in yet) " +
		"redeems it: only while they are still admin or owner, with a confirmed email address, unless switched off.", "admin", toAdmins},
	"test": {"Owner tools", "Only the owner can send it, to any address.", "", toOwner},
}

// Samples has every template in English and German, with the demo's people.
// picture is a JPEG the comment sample to the creator shows as the author's
// picture; the other comment samples show the initial. photos are mail
// photos of the dishes in dishes, in that order, and the first two are what
// the comment sample with photos shows; a missing one is a card
// without photo. TestEveryTemplateHasASample fails for a template missing
// here.
func Samples(publicURL string, picture []byte, photos [][]byte) []Sample {
	people := map[user.Locale]struct{ admin, invited, recipe, comment string }{
		"en": {"Damson Dill", "Noah Nutmeg", "Lentil dal with spinach",
			"Swapped the spinach for chard - great!\nA little less chili next time, the kids found it too hot."},
		"de": {"Dattel Dill", "Noah Nelke", "Linsen-Dal mit Spinat",
			"Hab statt Spinat Mangold genommen – super!\nBeim nächsten Mal etwas weniger Chili, den Kindern war's zu scharf."},
	}
	dishes := map[user.Locale][]string{
		"en": {"Beef and ale stew", "Full English breakfast", "Lemon drizzle cake", "Macaroni cheese",
			"Oven fish and chips", "Sausage rolls", "Shepherd's pie", "Sticky toffee pudding"},
		"de": {"Rindfleischeintopf mit Ale", "Englisches Frühstück", "Zitronenkuchen mit Guss", "Makkaroni mit Käsesoße",
			"Fisch und Pommes aus dem Ofen", "Würstchen im Blätterteig", "Hackfleischauflauf mit Kartoffelhaube", "Dattelpudding mit Karamellsoße"},
	}
	links := func(kind string) Unsubscribe {
		q := "?u=sample&k=" + kind + "&s=sample"
		return Unsubscribe{
			KindURL:     publicURL + "/unsubscribe" + q,
			GroupURL:    publicURL + "/unsubscribe" + q + "&scope=group",
			OneClickURL: publicURL + "/api/v1/unsubscribe/one-click" + q,
		}
	}
	var out []Sample
	for _, locale := range []user.Locale{"en", "de"} {
		p := people[locale]
		variant := func(template, variant string, build func(Config, time.Time) (Envelope, error)) {
			a := audiences[template]
			out = append(out, Sample{Template: template, Variant: variant, Locale: locale, Section: a.section, Audience: a.who, OptOut: a.optOut, Roles: a.roles, Build: build})
		}
		add := func(template string, build func(Config, time.Time) (Envelope, error)) {
			variant(template, "default", build)
		}
		for _, v := range []struct {
			name     string
			existing bool
		}{{"new", false}, {"existing", true}} {
			variant("invite", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				return BuildInvite(cfg, Invite{To: "noah@example.org", Name: p.invited, Inviter: p.admin, Path: "/welcome#sample", Locale: locale, Existing: v.existing}, publicURL, now)
			})
		}
		add("reset", func(cfg Config, now time.Time) (Envelope, error) {
			return BuildReset(cfg, Reset{To: "mila@example.org", Username: "mila", Path: "/welcome#sample", Locale: locale}, publicURL, now)
		})
		add("hint", func(cfg Config, now time.Time) (Envelope, error) {
			return BuildHint(cfg, Hint{To: "jonas@example.org", Provider: "Pocket ID", Locale: locale}, publicURL, now)
		})
		for _, v := range []struct{ name, admin string }{{"admin", p.admin}, {"self", ""}} {
			variant("confirm", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				return BuildConfirm(cfg, Confirm{To: "mila@example.org", Username: "mila", Admin: v.admin, Path: "/confirm-email#sample", Locale: locale}, publicURL, now)
			})
		}
		passwordNotice := func(name string, origin PasswordOrigin, first bool) {
			admin := ""
			if origin == OriginAdmin || origin == OriginSetupLink {
				admin = p.admin
			}
			variant("password_changed", name, func(cfg Config, now time.Time) (Envelope, error) {
				to, username, owner := "mila@example.org", "mila", false
				if origin == OriginCommandLine {
					// Only the owner's password is set on the command line.
					to, username, owner = "demo@example.org", "demo", true
				}
				return BuildPasswordChanged(cfg, PasswordChanged{To: to, Username: username, Admin: admin, Origin: origin, At: now, Locale: locale, Owner: owner, First: first}, publicURL, now)
			})
		}
		for _, origin := range []PasswordOrigin{OriginProfile, OriginAdmin, OriginResetLink, OriginSetupLink, OriginCommandLine} {
			passwordNotice(string(origin), origin, false)
		}
		for _, origin := range FirstOrigins {
			passwordNotice(string(origin)+"_first", origin, true)
		}
		for _, v := range []struct{ name, newAddress, admin string }{
			{"changed", "mila@example.net", ""}, {"removed", "", ""},
			{"admin_changed", "mila@example.net", p.admin}, {"admin_removed", "", p.admin},
		} {
			variant("email_changed", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				return BuildEmailChanged(cfg, EmailChanged{To: "mila@example.org", Username: "mila", Admin: v.admin, NewAddress: v.newAddress, At: now, Locale: locale}, publicURL, now)
			})
		}
		for _, v := range []struct {
			name   string
			origin IdentityOrigin
			admin  string
			owner  bool
		}{{"profile", IdentityProfile, "", false}, {"setup_link", IdentitySetupLink, p.admin, false}, {"owner", IdentityProfile, "", true}} {
			variant("identity_linked", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				to, username := "mila@example.org", "mila"
				if v.owner {
					to, username = "demo@example.org", "demo"
				}
				return BuildIdentityLinked(cfg, IdentityLinked{To: to, Username: username, Admin: v.admin, Provider: "Pocket ID", Origin: v.origin, At: now, Locale: locale, Owner: v.owner}, publicURL, now)
			})
		}
		for _, v := range []struct {
			name   string
			reason CommentReason
			photos [][]byte
		}{{"created", ReasonCreated, nil}, {"commented", ReasonCommented, nil}, {"photos", ReasonCommented, photos[:min(2, len(photos))]}} {
			var avatar []byte
			if v.reason == ReasonCreated {
				avatar = picture
			}
			variant("comment", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				return BuildComment(cfg, Comment{To: "mila@example.org", RecipeTitle: p.recipe, RecipeURLPath: "/recipes/sample?tab=comments",
					AuthorName: p.invited, AuthorColor: "sage", AuthorAvatar: avatar, Body: p.comment, Photos: v.photos, At: now, Reason: v.reason,
					Unsubscribe: links("comment"), Locale: locale}, publicURL, now)
			})
		}
		dish := func(i int) DigestRecipe {
			r := DigestRecipe{Title: dishes[locale][i], URLPath: "/recipes/sample", CreatorName: p.invited, CreatorColor: "sage",
				CreatorAvatar: picture, Servings: 2 + i%4, Minutes: 25 + 20*i,
				CreatedAt: time.Date(2026, 10, 27, 9+i, 15, 0, 0, time.Local)}
			if i%2 == 1 {
				r.CreatorName, r.CreatorColor, r.CreatorAvatar = p.admin, "clay", nil
			}
			if i < len(photos) {
				r.Photo = photos[i]
			}
			return r
		}
		for _, v := range []struct {
			name    string
			recipes []DigestRecipe
		}{
			{"one", []DigestRecipe{dish(0)}},
			{"no_photo", []DigestRecipe{func() DigestRecipe { r := dish(1); r.Photo, r.Minutes = nil, 0; return r }()}},
			{"many", func() []DigestRecipe {
				var rs []DigestRecipe
				for i := range dishes[locale] {
					rs = append(rs, dish(i))
				}
				rs[3].Photo = nil // a card without photo among the others
				return rs
			}()},
		} {
			variant("recipe_digest", v.name, func(cfg Config, now time.Time) (Envelope, error) {
				return BuildRecipeDigest(cfg, RecipeDigest{To: "mila@example.org", Recipes: v.recipes,
					Unsubscribe: links("recipe"), Locale: locale}, publicURL, now)
			})
		}
		add("invite_accepted", func(cfg Config, now time.Time) (Envelope, error) {
			return BuildInviteAccepted(cfg, InviteAccepted{To: "demo@example.org", Username: "noah", DisplayName: p.invited,
				Unsubscribe: links("invite_accepted"), Locale: locale}, publicURL, now)
		})
		add("test", func(cfg Config, now time.Time) (Envelope, error) {
			return BuildTest(cfg, "demo@example.org", locale, publicURL, now)
		})
	}
	return out
}
