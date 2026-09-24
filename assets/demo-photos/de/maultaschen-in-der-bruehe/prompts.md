# Maultaschen in der Brühe: demo photos

AI-generated originals for the sample recipe "Maultaschen in der Brühe", kept exactly as the generator returned them. `1.jpg` is the cover.

## Settings

Every photo in this folder was generated with these settings. To make an adjusted version, reuse them and change only the prompt.

| Setting       | Value                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------- |
| Tool          | `generate_image` of the [`mcp-image`](https://www.npmjs.com/package/mcp-image) MCP server, version 0.14.0 |
| Provider      | `gemini` (server default)                                                                                 |
| Model         | `gemini-3.1-flash-image`                                                                                  |
| `quality`     | `balanced`                                                                                                |
| `aspectRatio` | `4:3`                                                                                                     |
| `imageSize`   | `2K` (returns 2400 × 1792 px)                                                                             |
| Output        | JPEG                                                                                                      |
| Generated     | 2026-09-24                                                                                                |

Photos 2 and 3 also pass `inputImagePath` pointing at `1.jpg` and `maintainCharacterConsistency: true`, so they keep the cover's table, light and tableware.

## 1.jpg: cover

**purpose**

> Cover photo for a recipe in a home cookbook app, shown 4:3 on the detail page and center-cropped to a square on recipe cards

**prompt**

> Homemade Swabian Maultaschen in der Brühe in a wide, shallow cream soup plate: three large rectangular pasta pockets with slightly ruffled pressed edges, lying in clear, golden, steaming vegetable broth with small droplets of fat on the surface, one Maultasche cut in half showing its green filling of spinach and meat, scattered with plenty of fresh chives, a few thin fried golden onion strips on top. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The soup plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. No cutlery in the plate or in the air. Plain unbranded tableware. Each item appears once. No text, no labels, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Maultaschen from the reference image, shot from a low side angle almost at plate height: one Maultasche cut in half lying in the clear golden broth, its cut face in sharp focus showing the thin, silky pasta layers wrapped around the moist, green-flecked filling of spinach and meat, steam rising, chives and a few golden fried onion strips floating in the broth, small fat droplets glistening on the surface. The rest of the soup plate and the window light fall into a soft, strongly blurred background. No cutlery in the shot. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: cooking in the broth

Reference: `1.jpg`, `maintainCharacterConsistency: true`

Five attempts at the shaping step (filling, rolling or folding the dough, cutting) all failed: the model drew a thick spiral roll like a strudel, pockets far smaller than the roll they came from, cut pockets still lying on the dough sheet, filling showing at ends that are pressed shut before cutting, and a knife floating above the table. Neither a step-by-step description of the folding nor an in-place edit fixed it. The kept photo shows the simmering step instead, which leaves nothing to misread.

**purpose**

> Third gallery photo (cooking step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image, three-quarter overhead angle. A cooking step for Swabian Maultaschen: a wide plain stainless steel pot of gently simmering, clear golden vegetable broth, with a little steam rising. In the broth float four whole, closed Maultaschen: flat rectangular pasta pockets, each about the size of a palm, exactly the same size and shape as the Maultaschen in the soup plate of the reference image, with smooth, pressed-together edges. They are completely closed and uncut, no filling is visible anywhere, only the smooth pale yellow pasta. A slotted spoon rests on the rim of the pot. Beside the pot a small wooden board with a small heap of freshly chopped chives and a small cream bowl of golden fried onions. Only these items, each appearing once. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no brand names, no hands, no people.
