# Shepherd's Pie: demo photos

AI-generated originals for the sample recipe "Shepherd's Pie", kept exactly as the generator returned them. `1.jpg` is the cover.

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

Photos 2 and 3 also pass `inputImagePath` pointing at `1.jpg` and `maintainCharacterConsistency: true`, so they keep the cover's table, light and dish.

## 1.jpg: cover

**purpose**

> Cover photo for a recipe in a home cookbook app, shown 4:3 on the detail page and center-cropped to a square on recipe cards

**prompt**

> Homemade shepherd's pie in a cream ceramic baking dish with handles, golden browned mashed potato top with fork ridges, one portion scooped out showing the minced lamb, carrot and pea filling in rich gravy. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The dish is centered and fills about 55% of the frame width, with generous empty table on all sides so it survives a square center crop. Clean dish with only a little baked-on edge. No text, no hands, no people.

## 2.jpg: served

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Second gallery photo (served portion) for a recipe in a home cookbook app, shown 4:3 and center-cropped to a square

**prompt**

> Same kitchen scene, same rustic wooden table, same window light and the same cream baking dish of shepherd's pie from the reference image, now slightly out of focus in the background. In the foreground, one generous portion of shepherd's pie served on a simple cream plate: the golden browned mashed potato crust on top, the minced lamb, carrot and pea filling with gravy spilling a little onto the plate, a few buttered peas on the side, a fork resting on the plate edge. Closer view, three-quarter angle, the plate centered and filling about 55% of the frame width with calm space around it so it survives a square center crop. Realistic home cooking photography, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 and center-cropped to a square

**prompt**

> Same kitchen, same rustic wooden table and the same soft window light as the reference image. A preparation step for shepherd's pie: the same cream ceramic baking dish with handles, filled with the cooked minced lamb, carrot and pea filling in gravy, half of it already covered with fluffy unbaked mashed potato, the fork ridges just drawn in. Next to the dish a pot of mashed potato with a wooden spoon in it. Three-quarter overhead angle, the baking dish centered and filling about 55% of the frame width with calm space around it so it survives a square center crop. Realistic home cooking photography, warm muted tones. No text, no hands, no people.
