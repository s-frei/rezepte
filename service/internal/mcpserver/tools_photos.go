package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/s-frei/rezepte/service/internal/auth"
	"github.com/s-frei/rezepte/service/internal/image"
	"github.com/s-frei/rezepte/service/internal/recipeimport"
)

// Photos is what the photo tools need beyond the recipe service. A photo's
// bytes never pass through the model: add_photo has the server fetch a URL,
// photo_upload_link hands out a signed link a client with a shell uploads
// a local file to.
type Photos struct {
	Images *image.Service
	Links  *image.UploadLinks
	// Fetch must refuse private addresses in production
	// (recipeimport.PublicOnly), or add_photo reaches the internal network.
	Fetch *recipeimport.Fetcher
}

func init() {
	tools = append(tools,
		tool{auth.ScopeRecipesRead, addGetPhoto},
		tool{auth.ScopeRecipesWrite, addAddPhoto},
		tool{auth.ScopeRecipesWrite, addPhotoUploadLink},
	)
}

const photoSteering = "The first photo a recipe gets becomes its cover, so add the best one first. " +
	"Only JPEG, PNG and WebP are accepted; convert anything else first (HEIC on macOS: sips -s format jpeg in.heic --out out.jpg). " +
	"Check the result with get_recipe, whose images and coverImageId list the photos."

type addPhotoIn struct {
	RecipeID string `json:"recipeId" jsonschema:"The recipe's id"`
	URL      string `json:"url" jsonschema:"An http or https address of the photo itself, not of a page"`
}

func addAddPhoto(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "add_photo",
		Description: "Add a photo from the web to a recipe: the server downloads it from url and returns the new photo. " +
			"For a file on this computer use photo_upload_link instead. " + photoSteering,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(true)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in addPhotoIn) (*mcp.CallToolResult, any, error) {
		got, err := c.photos.Fetch.Get(ctx, in.URL, "image/jpeg,image/png,image/webp", image.MaxUploadBytes)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("fetch photo: %w", err))
		}
		img, err := c.photos.Images.Upload(ctx, in.RecipeID, c.user, bytes.NewReader(got.Body))
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("add photo: %w", err))
		}
		return nil, img, nil
	})
}

type uploadLinkIn struct {
	RecipeID string `json:"recipeId" jsonschema:"The recipe's id"`
}

type uploadLinkOut struct {
	Curl      string    `json:"curl" jsonschema:"The upload command; replace <path> with the photo's path and run it once per photo"`
	ExpiresAt time.Time `json:"expiresAt" jsonschema:"When the link stops working; ask for a new one after that"`
}

func addPhotoUploadLink(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "photo_upload_link",
		Description: "Get a link that uploads photos from this computer to a recipe, for an assistant that can run shell commands. " +
			"Run the returned curl command once per photo; the link works for 15 minutes and needs no other credential. " +
			"For a photo on the web use add_photo instead. " + photoSteering,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in uploadLinkIn) (*mcp.CallToolResult, any, error) {
		link, expires, err := c.photos.Links.Issue(in.RecipeID, c.user.ID)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("issue upload link: %w", err))
		}
		return nil, uploadLinkOut{Curl: "curl -F file=@<path> '" + link + "'", ExpiresAt: expires}, nil
	})
}

type getPhotoIn struct {
	RecipeID string `json:"recipeId" jsonschema:"The recipe's id"`
	PhotoID  string `json:"photoId" jsonschema:"The photo's id, from images in get_recipe"`
}

func addGetPhoto(s *mcp.Server, c caller) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_photo",
		Description: "Look at one of a recipe's photos: returns it as a JPEG at most 960 px on its longest side. Photo ids are in images in get_recipe.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getPhotoIn) (*mcp.CallToolResult, any, error) {
		f, err := c.photos.Images.Open(in.RecipeID, in.PhotoID, "thumb")
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("open photo: %w", err))
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, nil, toolError(ctx, fmt.Errorf("read photo: %w", err))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.ImageContent{Data: data, MIMEType: "image/jpeg"}}}, nil, nil
	})
}
