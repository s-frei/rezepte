// Package pdf renders recipes to PDF with Typst. data.go owns the data.json
// contract that templates read.
package pdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// SchemaVersion is bumped only when a field is removed or changes meaning.
const SchemaVersion = 1

// Bundle is part of the data.json contract.
type Bundle string

// Enum values of the data.json contract.
const (
	BundleSingle Bundle = "single"
	BundleBook   Bundle = "book"
	BundleZip    Bundle = "zip"
)

// Paper is part of the data.json contract.
type Paper string

// Enum values of the data.json contract.
const (
	PaperA4 Paper = "a4"
	PaperA5 Paper = "a5"
)

// Generator is part of the data.json contract.
type Generator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Options is part of the data.json contract.
type Options struct {
	Bundle Bundle `json:"bundle" enum:"single,book,zip"`
	Paper  Paper  `json:"paper" enum:"a4,a5"`
	Photo  bool   `json:"photo" doc:"Whether to show the photo; recipes[].photo is sent either way"`
	Footer bool   `json:"footer" doc:"The QR code and the credit line; the source prints regardless"`
}

// Instance is part of the data.json contract.
type Instance struct {
	Product     string `json:"product" doc:"The product name for the credit line, never an operator's text"`
	URL         string `json:"url"`
	Attribution bool   `json:"attribution"`
}

// Labels are the captions printed by the template, in the UI language.
type Labels struct {
	Servings       string `json:"servings"`
	Prep           string `json:"prep"`
	Cook           string `json:"cook"`
	Total          string `json:"total"`
	Ingredients    string `json:"ingredients"`
	Steps          string `json:"steps"`
	Source         string `json:"source"`
	AddedBy        string `json:"addedBy"`
	Contents       string `json:"contents"`
	RecipeFallback string `json:"recipeFallback" doc:"The file name of a recipe without a title"`
	Continued      string `json:"continued"`
	NextPage       string `json:"nextPage"`
	OpenRecipe     string `json:"openRecipe"`
	// ScaledFrom and ScaledFromShort note servings printed at another count
	// than the recipe's ("scaled from {n}", "from {n}"); the template puts the
	// recipe's own servings in for {n}.
	ScaledFrom      string `json:"scaledFrom"`
	ScaledFromShort string `json:"scaledFromShort"`
}

// Book is part of the data.json contract. CountDisplay is the recipe count
// as the title page prints it ("12 recipes"), formatted by the client.
type Book struct {
	_            struct{} `nullable:"true"`
	Title        string   `json:"title" minLength:"1"`
	Date         string   `json:"date" format:"date"`
	DateDisplay  string   `json:"dateDisplay"`
	CountDisplay string   `json:"countDisplay"`
}

// Servings is part of the data.json contract.
type Servings struct {
	Value    int    `json:"value"`
	Original int    `json:"original"`
	Display  string `json:"display"`
}

// Duration is part of the data.json contract.
type Duration struct {
	_       struct{} `nullable:"true"`
	Minutes int      `json:"minutes"`
	Display string   `json:"display"`
}

// Times is part of the data.json contract.
type Times struct {
	Prep  *Duration `json:"prep"`
	Cook  *Duration `json:"cook"`
	Total *Duration `json:"total"`
}

// Source is part of the data.json contract. Display is Name, else the host
// of URL without a leading "www.".
type Source struct {
	_       struct{} `nullable:"true"`
	Name    *string  `json:"name" nullable:"true"`
	URL     *string  `json:"url" nullable:"true"`
	Display string   `json:"display" doc:"How the source prints: the name, else the host of the URL without a leading www."`
}

// Author is part of the data.json contract.
type Author struct {
	DisplayName string `json:"displayName"`
}

// Focus is part of the data.json contract.
type Focus struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Photo is part of the data.json contract.
type Photo struct {
	_      struct{} `nullable:"true"`
	Path   string   `json:"path"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Focus  Focus    `json:"focus"`
}

// QR is part of the data.json contract.
type QR struct {
	_    struct{} `nullable:"true"`
	URL  string   `json:"url"`
	Path string   `json:"path" doc:"The code as an SVG without a quiet zone: leave four modules of white around it"`
}

// Quantity is part of the data.json contract.
type Quantity struct {
	_       struct{} `nullable:"true"`
	Value   float64  `json:"value"`
	Display string   `json:"display"`
}

// PDFIngredient is part of the data.json contract; the prefix keeps its
// API schema name apart from recipe.Ingredient.
type PDFIngredient struct { //nolint:revive // the prefix is the API schema name, apart from recipe.Ingredient
	Quantity *Quantity `json:"quantity"`
	Unit     *string   `json:"unit" nullable:"true"`
	Name     string    `json:"name"`
	Note     *string   `json:"note" nullable:"true"`
	Display  string    `json:"display"`
}

// Group is part of the data.json contract.
type Group struct {
	Name        *string         `json:"name" nullable:"true"`
	Ingredients []PDFIngredient `json:"ingredients" nullable:"false"`
}

// Segment kinds: text, ingredient (with Amount), time (with Minutes).
type Segment struct {
	Kind    string  `json:"kind" enum:"text,ingredient,time"`
	Text    string  `json:"text"`
	Amount  *string `json:"amount" nullable:"true"`
	Minutes *int    `json:"minutes" nullable:"true"`
}

// PDFStep is part of the data.json contract; the prefix keeps its API
// schema name apart from recipe.Step.
type PDFStep struct { //nolint:revive // the prefix is the API schema name, apart from recipe.Step
	Number   int       `json:"number"`
	Text     string    `json:"text"`
	Segments []Segment `json:"segments" nullable:"false"`
}

// Recipe is part of the data.json contract.
type Recipe struct {
	ID               string    `json:"id"`
	Slug             string    `json:"slug"`
	URL              string    `json:"url"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Tags             []string  `json:"tags" nullable:"false"`
	Servings         Servings  `json:"servings"`
	Times            Times     `json:"times"`
	Source           *Source   `json:"source"`
	Author           Author    `json:"author"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	Photo            *Photo    `json:"photo"`
	QR               *QR       `json:"qr"`
	IngredientGroups []Group   `json:"ingredientGroups" nullable:"false"`
	Steps            []PDFStep `json:"steps" nullable:"false"`
}

// Data is part of the data.json contract.
type Data struct {
	SchemaVersion int       `json:"schemaVersion"`
	Generator     Generator `json:"generator"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Locale        string    `json:"locale"`
	Options       Options   `json:"options"`
	Instance      Instance  `json:"instance"`
	Labels        Labels    `json:"labels"`
	Book          *Book     `json:"book"`
	Recipes       []Recipe  `json:"recipes" nullable:"false"`
}

// ClientRecipe is what the browser formats and sends.
type ClientRecipe struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Tags             []string  `json:"tags" nullable:"false"`
	Servings         Servings  `json:"servings"`
	Times            Times     `json:"times"`
	Source           *Source   `json:"source"`
	IngredientGroups []Group   `json:"ingredientGroups" nullable:"false"`
	Steps            []PDFStep `json:"steps" nullable:"false"`
}

// ServerRecipe is what only the server knows.
type ServerRecipe struct {
	Slug      string
	URL       string
	Author    Author
	CreatedAt time.Time
	UpdatedAt time.Time
	Photo     *Photo
}

// Merge joins both halves; nil slices become empty so JSON never has null lists.
func Merge(c ClientRecipe, s ServerRecipe) Recipe {
	r := Recipe{
		ID: c.ID, Slug: s.Slug, URL: s.URL, Title: c.Title, Description: c.Description,
		Tags: c.Tags, Servings: c.Servings, Times: c.Times, Source: c.Source,
		Author: s.Author, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
		Photo: s.Photo, IngredientGroups: c.IngredientGroups, Steps: c.Steps,
	}
	if r.Tags == nil {
		r.Tags = []string{}
	}
	if r.IngredientGroups == nil {
		r.IngredientGroups = []Group{}
	}
	for i := range r.IngredientGroups {
		if r.IngredientGroups[i].Ingredients == nil {
			r.IngredientGroups[i].Ingredients = []PDFIngredient{}
		}
	}
	if r.Steps == nil {
		r.Steps = []PDFStep{}
	}
	for i := range r.Steps {
		if r.Steps[i].Segments == nil {
			r.Steps[i].Segments = []Segment{}
		}
	}
	return r
}

// WriteSchema writes the standalone JSON Schema (draft 2020-12) for Data.
func WriteSchema(w io.Writer) error {
	reg := huma.NewMapRegistry("#/$defs/", huma.DefaultSchemaNamer)
	root := reg.Schema(reflect.TypeFor[Data](), true, "Data")
	// encoding/json sorts map keys, so the output is stable.
	out := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs":   reg.Map(),
	}
	b, err := json.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal root schema: %w", err)
	}
	var rootMap map[string]any
	if err := json.Unmarshal(b, &rootMap); err != nil {
		return fmt.Errorf("decode root schema: %w", err)
	}
	for k, v := range rootMap {
		out[k] = v
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode schema: %w", err)
	}
	_, err = w.Write(buf.Bytes())
	return err
}
