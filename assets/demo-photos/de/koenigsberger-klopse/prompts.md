# Königsberger Klopse: demo photos

AI-generated originals for the sample recipe "Königsberger Klopse", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade German dish, Königsberger Klopse, served on a cream plate: four tender, pale poached meatballs in a creamy, glossy white caper sauce with plenty of capers, a sprinkle of fresh chopped parsley and a thin slice of lemon, boiled potatoes and a few slices of pickled beetroot beside them for colour. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The plate is small in the frame: it fills about 50% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. No text, no labels, no hands, no people.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Königsberger Klopse from the reference image, shot from a low side angle almost at plate height: one tender meatball cut in half with a fork, showing its juicy, fine-textured inside, coated in the creamy white caper sauce with whole capers, fresh parsley and a glint of lemon, a halved boiled potato soaking up sauce beside it. The rest of the plate, the beetroot and the window light fall into a soft, strongly blurred background. The fork rests on the plate rim, nobody holds it. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

The fork still came out floating above the plate; to avoid that, leave cutlery out of the shot.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for Königsberger Klopse, three-quarter overhead angle: a wide plain stainless steel pot of gently simmering clear beef broth with six pale round meatballs poaching in it, a slotted spoon resting on the rim. Next to the pot a board with a few raw shaped meatballs still waiting, a small bowl of capers, a halved lemon, a small jug of cream and a small bowl of flour with a piece of butter. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no hands, no people.
