package recipeimport

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/s-frei/rezepte/service/internal/recipe"
	"github.com/s-frei/rezepte/service/internal/sign"
)

const (
	pageLimit  = 5 << 20
	photoLimit = 10 << 20
	photoTTL   = 2 * time.Minute
)

// The messages of the first two are the reasons the API reports.
var (
	ErrNoRecipe       = errors.New("no-recipe")
	ErrNoRecipeInText = errors.New("no-recipe-in-text")
	ErrPhotoNotFound  = errors.New("photo not found")
)

// Result is a draft and what the service found around it.
type Result struct {
	Draft     Draft
	Duplicate *recipe.SourceMatch
}

// Service builds drafts.
type Service struct {
	recipes *recipe.Service
	fetch   *Fetcher
	photos  *sign.Signer
}

// NewService draws a fresh photo-signing key; tokens live two minutes, so
// one that does not survive a restart costs at most a photo.
func NewService(recipes *recipe.Service, version string, allow func(netip.Addr) bool) *Service {
	key := make([]byte, 32)
	_, _ = rand.Read(key) // crypto/rand.Read never fails on supported platforms
	return &Service{recipes: recipes, fetch: NewFetcher(version, allow), photos: sign.New(key, "rezepte draft photo")}
}

// FromURL fetches and reads a recipe page.
func (s *Service) FromURL(ctx context.Context, raw string) (Result, error) {
	got, err := s.fetch.Get(ctx, raw, "text/html,application/xhtml+xml", pageLimit)
	if err != nil {
		return Result{}, err
	}
	if ct := strings.ToLower(got.ContentType); ct != "" && !strings.Contains(ct, "html") {
		return Result{}, fmt.Errorf("%w: content type %q", ErrUnreachable, ct)
	}
	dec, err := charset.NewReader(bytes.NewReader(got.Body), got.ContentType) // "K\xe4se" in Latin-1 is "Käse"
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	body, err := io.ReadAll(dec)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	page, ok := ReadPage(body, got.URL)
	if !ok {
		return Result{}, ErrNoRecipe
	}
	r, err := s.build(ctx, page)
	if err != nil {
		return Result{}, err
	}
	r.Duplicate, err = s.recipes.FindBySource(ctx, []string{raw, got.URL.String(), page.SourceURL})
	return r, err
}

// FromText reads pasted text.
func (s *Service) FromText(ctx context.Context, text string) (Result, error) {
	page, ok := ReadText(text)
	if !ok {
		return Result{}, ErrNoRecipeInText
	}
	return s.build(ctx, page)
}

func (s *Service) build(ctx context.Context, page Page) (Result, error) {
	counts, err := s.recipes.Tags(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load tags: %w", err)
	}
	known := make([]string, len(counts))
	for i, c := range counts {
		known[i] = c.Name
	}
	return Result{Draft: Build(page, known)}, nil
}

// PhotoHref is the address the browser redeems for the draft's photo.
func (s *Service) PhotoHref(src string) string {
	token, _ := s.photos.Sign(src, photoTTL)
	return "/api/v1/recipe-drafts/photo?" + url.Values{"src": {src}, "token": {token}}.Encode()
}

// Photo fetches a photo the service itself found, proven by token.
func (s *Service) Photo(ctx context.Context, src, token string) ([]byte, string, error) {
	if !s.photos.Verify(src, token) {
		return nil, "", ErrPhotoNotFound
	}
	got, err := s.fetch.Get(ctx, src, "image/jpeg,image/png,image/webp", photoLimit)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrPhotoNotFound, err)
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(got.ContentType, ";")[0]))
	switch ct {
	case "image/jpeg", "image/png", "image/webp":
		return got.Body, ct, nil
	}
	return nil, "", fmt.Errorf("%w: content type %q", ErrPhotoNotFound, ct)
}
