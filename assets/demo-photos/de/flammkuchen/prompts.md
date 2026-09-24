# Flammkuchen: demo photos

AI-generated originals for the sample recipe "Flammkuchen", kept exactly as the generator returned them. `1.jpg` is the cover.

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

> A homemade Alsatian Flammkuchen fresh from the oven on a rectangular wooden serving board: a very thin, crisp, rectangular base with blistered, charred golden edges, spread with white crème fraîche, topped with thin rings of soft onion and small crispy bacon cubes, a scattering of fresh chives, one strip already cut into pieces with a pizza wheel lying on the board. Style: realistic home cooking photography, rustic wooden table, soft natural side light from a window, warm muted tones, three-quarter overhead angle, shallow depth of field. The board is small in the frame: it fills about 55% of the frame width, fully inside the frame with generous empty table on all sides so it survives a square center crop. Plain unbranded tableware. Each item appears once. No text, no labels, no hands, no people.

The board came out larger than asked; the square card crop trims it on both sides and a thin strip of the Flammkuchen on the left.

## 2.jpg: close-up

Reference: `1.jpg`, `maintainCharacterConsistency: true`

A first attempt asked for a piece "lifted slightly on the edge of the board"; it came out folded up like a tent, showing only its underside. Keep every piece lying flat.

**purpose**

> Second gallery photo (appetizing close-up detail) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Close-up of the Flammkuchen from the reference image, shot from a low side angle almost at board height: two cut rectangular pieces lying flat on the wooden board, slightly apart from the rest, their cut edges facing the camera. In sharp focus: the paper-thin, crisp base with blistered, charred spots along its edge, the creamy white crème fraîche layer on top of it, glossy soft onion rings, crispy bacon cubes and bright green chives. The rest of the Flammkuchen and the window light fall into a soft, strongly blurred background. Every piece lies flat on the board, nothing is lifted, folded or standing up. No cutlery in the shot. Same rustic wooden table and warm window light as the reference. Macro food photography, shallow depth of field, realistic home cooking, warm muted tones. No text, no hands, no people.

## 3.jpg: preparation

Reference: `1.jpg`, `maintainCharacterConsistency: true`

**purpose**

> Third gallery photo (preparation step) for a recipe in a home cookbook app, shown 4:3 in the gallery

**prompt**

> Same rustic wooden table and the same soft window light as the reference image. A preparation step for Flammkuchen, three-quarter overhead angle: a very thin, rolled-out rectangle of raw pale dough lying on baking paper on a dark metal baking tray, half of it already spread with white crème fraîche, a spoon resting in a small bowl of crème fraîche beside it. Next to the tray a board with a halved onion and thin raw onion rings, a small bowl of diced raw bacon, and a wooden rolling pin with a dusting of flour on the table. Only these items, each appearing once, with a little natural mess of flour. Realistic home cooking photography, warm muted tones. Plain unbranded cookware. No text, no labels, no brand names, no hands, no people.
