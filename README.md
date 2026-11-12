<div align="center">

<img alt="Rezepte app icon: the waving cookbook mascot with a wooden spoon" src="assets/readme/app-icon.png" width="160">

# Rezepte

**A self-hosted recipe manager for the people you cook with.**<br>
Your family's and friends' recipes in one place, on your own server, without a cloud account.

[![CI](https://github.com/s-frei/rezepte/actions/workflows/ci.yml/badge.svg)](https://github.com/s-frei/rezepte/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/s-frei/rezepte)](https://github.com/s-frei/rezepte/releases/latest)
[![Documentation](https://img.shields.io/badge/docs-getrezepte.app-a2653e)](https://getrezepte.app/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

[Documentation](https://getrezepte.app/) · [Getting started](https://getrezepte.app/getting-started/) · [Features](https://getrezepte.app/guide/) · [Changelog](https://getrezepte.app/changelog/) · [API](https://getrezepte.app/api/) · [Roadmap](https://getrezepte.app/roadmap/)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/readme/overview-desktop-dark.png">
  <img alt="Rezepte recipe overview on a desktop: search, tag chips and recipe cards in columns, most of them with a photo" src="assets/readme/overview-desktop-light.png" width="74%">
</picture>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/readme/cook-mode-timers-mobile-dark.png">
  <img alt="Rezepte cook mode on a phone: a step in large type with its time running as a timer, and two timers counting down below it" src="assets/readme/cook-mode-timers-mobile-light.png" width="21%">
</picture>

</div>

*Rezepte* is German for "recipes". You run it once, invite your household and your friends with a link or by mail, and everyone writes down what they cook: ingredient groups, steps, photos, where the recipe comes from. Then you find it again by title, ingredient or tag, put its ingredients on a shopping list, and cook from your phone at the stove. The interface is modern and made for the phone first: light and dark, a bottom bar under your thumb, <kbd>⌘</kbd><kbd>K</kbd> to jump anywhere, and an [app to install](https://getrezepte.app/guide/install-app/) on your home screen.

It runs as **one binary** with the web app built in, an SQLite database and your photos in a single data directory: no Node runtime, no database server, no external services.

## Try it

Demo mode fills an empty instance with twelve sample recipes, most of them with AI-generated photos, and a small household with comments and shared shopping lists, so you can click through the real app without setting anything up:

```bash
docker run --rm -p 8060:8060 ghcr.io/s-frei/rezepte:latest --demo
```

Open <http://localhost:8060> and log in as **`demo`** / **`demo1234`**. The log prints the other logins too: sign in as **`mila`** / **`mila1234`** in a private window and tick an item on "Weekly shop", which you share with her, to see it change on both screens. Or choose "Sign in another person" in the account menu, sign in as her, and switch between the two of you with a tap under "Who's cooking?". On "Sausage Rolls", "brush the tops with the remaining egg" in the fourth step opens the photo it describes.

> [!NOTE]
> Nothing is kept: there is no volume, and `--rm` removes the container when it stops. To keep what you write, [run your own instance](#run-your-own-instance).

## Features

- 🍳 **Cooking:** a full-screen [cook mode](https://getrezepte.app/guide/cook-mode/) for the phone at the stove, steps that show the amounts they use, [times in a step](https://getrezepte.app/guide/recipes/#mark-times-in-a-step) that start [a timer](https://getrezepte.app/guide/cook-mode/#run-timers) with a tap and ring until you turn it off, with a notification when the phone is locked, [photos a step opens](https://getrezepte.app/guide/recipes/#link-a-photo-in-a-step) right at the words that describe them, quantities [scaled to your servings](https://getrezepte.app/guide/servings/), and what you have not ticked off [copied as a list](https://getrezepte.app/guide/servings/#copy-a-shopping-list) for another app.
- 🛒 **Shopping lists:** [lists](https://getrezepte.app/guide/shopping-lists/) filled from recipes at the servings you choose and by hand, sorted by store department, with the same ingredient from several recipes added up into one row, or [pasted in as text](https://getrezepte.app/guide/shopping-lists/#import-a-list) from a note or an AI chat. [Share a list](https://getrezepte.app/guide/shopping-lists/#share-a-list) with anyone on your instance and every open screen [follows along live](https://getrezepte.app/guide/shopping-lists/#live); in the store, [shopping mode](https://getrezepte.app/guide/shopping-lists/#shopping-mode) keeps the screen on and ticks a row with one tap.
- 📱 **An app on your phone:** [install Rezepte](https://getrezepte.app/guide/install-app/) on the home screen or the desktop, no app store needed. It opens in its own window with [shortcuts](https://getrezepte.app/guide/install-app/#shortcuts) to the import, a new recipe and your favorites, and tells you when a timer is done, even with the phone locked. On Android, Windows and ChromeOS it joins the [share menu](https://getrezepte.app/guide/install-app/#share-to-rezepte): share a recipe link from another app or a chat, and it lands straight in the import.
- 📖 **Collection:** [recipes](https://getrezepte.app/guide/recipes/) with ingredient groups, notes, times and their source (a book, a website or a person), [several photos each](https://getrezepte.app/guide/images/) in the order you drag them into, the first one the cover, [tags and search](https://getrezepte.app/guide/tags-and-search/) by ingredient, time or author, private favorites, [tasty marks](https://getrezepte.app/guide/recipes/#tasty) that show who liked what, and [comments](https://getrezepte.app/guide/comments/) with photos for what you learned cooking it.
- 🔔 **Notifications:** [by mail, as a push, or both](https://getrezepte.app/guide/notifications/) when someone comments on a recipe you follow, finds yours tasty or accepts your invitation, and once a day about the recipes the others added. Every member [chooses what comes](https://getrezepte.app/guide/notifications/#choose-what-comes) on which channel, every notification mail has a one-click unsubscribe, and the owner gets a push when mail stops getting out.
- 👥 **People:** [accounts and roles](https://getrezepte.app/guide/users/) with a one-time setup link, [sent by mail](https://getrezepte.app/guide/mail/) if you connect a mail server, and [Forgot password?](https://getrezepte.app/getting-started/first-login/#forgot-your-password) on the login page, everyone on a card with their newest recipes, [one shared kitchen tablet for the whole household](https://getrezepte.app/guide/settings/#share-a-device) where everyone stays signed in and switches with a tap, a [profile](https://getrezepte.app/guide/settings/#profile) with your photo and a color that marks you across the app, [sign-in with Google or your own OIDC provider](https://getrezepte.app/operations/single-sign-on/) set up right in the app, and [sharing](https://getrezepte.app/guide/sharing/) as a household link, a [public link](https://getrezepte.app/guide/sharing/#public-links) that expires when you say, or [an image for any messenger](https://getrezepte.app/guide/sharing/#pass-on-as-an-image).
- 📥 **Import:** [paste a link or the recipe text](https://getrezepte.app/guide/importing/), or share a link into the installed app, and get a draft to check, with ingredients split into amounts, units and names, the photo and the source; anything Rezepte was unsure about is marked for you. Or let ChatGPT, Claude or any other AI chat [write the recipe](https://getrezepte.app/guide/text-format/): copy its instruction from the import, paste the answer back, and it arrives complete.
- 🔌 **Your data:** [zip export and import](https://getrezepte.app/guide/import-export/) between instances, an [HTTP API](https://getrezepte.app/api/) with an OpenAPI reference and [tokens](https://getrezepte.app/api/tokens/) that get their own level for recipes, accounts and shopping lists, and an [MCP server](https://getrezepte.app/api/mcp/) through which AI assistants read and edit your recipes and fill your shopping lists.

English and German are built in, and every member picks their own. Missing your language? [Adding one](https://getrezepte.app/contributing/add-a-language/) takes four files and no programming.

<details>
<summary><b>What an AI assistant can do through MCP</b></summary>

<br>

Connect Claude Code, Cursor, VS Code or any MCP client that sends a token header. The assistant gets twenty-two tools, each limited by what the token allows:

| Token level | Tools |
| --- | --- |
| **Recipes: Read** | search recipes, read a recipe and its comments, list the tags in use, look at a recipe's photos |
| **Recipes: Write** | create a recipe, edit a recipe, add photos from a link or from your computer, write a comment, star or unstar it, mark it tasty or take the mark back |
| **Recipes: Delete** | delete a recipe with its photos |
| **Shopping lists: Read** | list your shopping lists and read one |
| **Shopping lists: Write** | create a list, put a recipe on it at the servings you name, add items, tick items off or remove them |

Nothing touches accounts or passwords, and sharing, renaming or deleting a list stays in the app. Clients that only connect through OAuth, such as claude.ai's custom connectors, are not supported. → [Connect an AI assistant](https://getrezepte.app/api/mcp/)

</details>

## Coming from Mealie, Tandoor or Paprika?

They are mature apps, and Rezepte is younger and smaller. Here is what it does well:

- **A modern interface made for the phone.** Cook mode, a bottom bar and the command palette are part of the design from the start, not added later.
- **AI assistants built in.** Any AI chat writes recipes and shopping lists that paste in complete, a first-party MCP server lets assistants work on them directly, and API tokens with their own level per area, from read-only up to delete, instead of a token that can do everything its owner can.
- **Sharing you stay in control of.** Public links expire and can be revoked one by one or all at once, link previews start switched off, and a recipe can travel as a plain image.
- **Invitation only.** Nobody can register on their own, and nobody gets an account just by having one at your sign-in provider.
- **One device for the whole kitchen.** Everyone stays signed in on a shared tablet and switches with a tap, each with their own favorites, lists and notifications.
- **Little to run.** No database server or runtime to look after, and an Apache 2.0 license.

What it does not do yet: import from other apps, plan meals, convert units, or show nutrition. If you need those today, Mealie and Tandoor do them well.

> [!TIP]
> None of these are ruled out. The [roadmap](https://getrezepte.app/roadmap/) lists what is being considered, and I am open to anything that helps people cook. If you are missing something, [share the idea in Discussions](https://github.com/s-frei/rezepte/discussions/categories/ideas) and tell me what you need.

## What to expect

- **Stable since 1.0.** Rezepte follows [semantic versioning](https://semver.org): only a new major version can ask something of you, and such a release says so at the top of its [changelog](https://getrezepte.app/changelog/) page.
- **Your data stays yours.** Everything lives in one directory: an SQLite database and plain JPEG files. [Back up the whole directory](https://getrezepte.app/operations/data-and-backup/) before every update, and use [zip export](https://getrezepte.app/guide/import-export/) to move recipes to another instance.
- **One maintainer.** I built Rezepte because I needed it myself. Issues get read, but there is no support guarantee.
- **Developed AI-driven.** Most of the code and all of the documentation is written by AI coding agents, working against the guidelines and the agent memory in this repository, and reviewed and steered by me. It is an experiment as much as a workflow, and a good part of why I enjoy building this.

## Run your own instance

Compose is the setup Rezepte is meant to be run with. Save this as `docker-compose.yaml`, choose your own owner password, and start it with `docker compose up -d`:

```yaml
services:
  rezepte:
    image: ghcr.io/s-frei/rezepte:latest
    restart: unless-stopped
    # Room to finish a running daily mail on stop: up to 10 s for open
    # requests plus up to 2 min for the mail, where Docker's default is 10 s.
    stop_grace_period: 2m30s
    volumes:
      - rezepte-data:/data
    environment:
      REZEPTE_ADMIN_PASSWORD: change-me-now
      # The time zone the times in mails are written in; UTC without it.
      # TZ: Europe/Berlin
    ports:
      - "8060:8060"

volumes:
  rezepte-data:
```

Open <http://localhost:8060> and log in as **`admin`** with the password you set. That account owns the instance. There is no self-registration, so invite everyone else from the settings once you are in.

> [!IMPORTANT]
> `REZEPTE_ADMIN_PASSWORD` is read only on the very first start, to create the owner. Afterwards, change the password inside the app; the variable is read again only by `--reset-superadmin-password`.

Before you invite anyone, put Rezepte behind a [reverse proxy with HTTPS](https://getrezepte.app/operations/reverse-proxy/): browsers keep the installable app, push notifications, timer alarms for a locked phone and a screen that stays on in cook and shopping mode to secure addresses.

`latest` is the newest stable release. Every version is on the [releases page](https://github.com/s-frei/rezepte/releases), and every image tag on the [package page](https://github.com/s-frei/rezepte/pkgs/container/rezepte); images and binaries both carry an SPDX SBOM for your vulnerability scanner. [Getting started](https://getrezepte.app/getting-started/) covers plain `docker run`, the binary with systemd, and every setting.

## Documentation

Everything lives at **[getrezepte.app](https://getrezepte.app/)**:

| Page | What you find there |
| --- | --- |
| [Getting started](https://getrezepte.app/getting-started/) | Run Rezepte with Docker or as a binary, and log in for the first time. |
| [Tour of the app](https://getrezepte.app/guide/) | Every feature, with screenshots. |
| [Operations](https://getrezepte.app/operations/data-and-backup/) | Where the data lives, backups, updates, reverse proxies and [single sign-on](https://getrezepte.app/operations/single-sign-on/). |
| [API](https://getrezepte.app/api/) | The HTTP API behind the app, with a generated reference, and the MCP server for AI assistants. |
| [Changelog](https://getrezepte.app/changelog/) | What changed in each release, and what to do before you update. |
| [Roadmap](https://getrezepte.app/roadmap/) | Where Rezepte could go next. |
| [Contributing](https://getrezepte.app/contributing/add-a-language/) | Translate the interface into another language. |

AI assistants can read the whole manual too: [`/llms.txt`](https://getrezepte.app/llms.txt) indexes it and [`/llms-full.txt`](https://getrezepte.app/llms-full.txt) is all of it in one file.

## Development

The stack is a Go service serving a bundled SvelteKit SPA, with SQLite for storage. Every tool is pinned, and every task runs through [mise](https://mise.jdx.dev):

```bash
mise trust && mise install && mise run setup   # tools and dependencies
mise run dev                                   # app on :9060, API on :8060
mise run check                                 # every lint, format and test gate
```

`mise run dev` creates a local `admin` / `admin1234` login on first start and prints it.

The long-form developer documentation (architecture, conventions and how-tos) lives in the repository under [`docs/memory/content/`](docs/memory/content/) and is not published.

## Contributing

Contributions are welcome. [Open an issue](https://github.com/s-frei/rezepte/issues) for a bug, and share an idea in [Discussions](https://github.com/s-frei/rezepte/discussions/categories/ideas). For a pull request, branch off `develop`, keep `mise run check` green, and write everything in English except the UI copy in `frontend/messages/`, which has one catalog per language.

A translation is the easiest way in: [Add a language](https://getrezepte.app/contributing/add-a-language/) walks through it, and it needs no code.

## License

[Apache License 2.0](LICENSE)
