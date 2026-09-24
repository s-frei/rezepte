<div align="center">

<h1>
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/lockups/rezepte-lockup-horizontal-dark.svg">
    <img alt="Rezepte" src="assets/brand/lockups/rezepte-lockup-horizontal.svg" height="72">
  </picture>
</h1>

**The recipe manager I wanted for my own kitchen.** Self-hosted, one binary, no cloud account.

[![CI](https://github.com/s-frei/rezepte/actions/workflows/ci.yml/badge.svg)](https://github.com/s-frei/rezepte/actions/workflows/ci.yml)
[![Documentation](https://img.shields.io/badge/docs-s--frei.github.io%2Frezepte-a2653e)](https://s-frei.github.io/rezepte/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

[Documentation](https://s-frei.github.io/rezepte/) · [Getting started](https://s-frei.github.io/rezepte/getting-started/) · [Features](https://s-frei.github.io/rezepte/guide/) · [API](https://s-frei.github.io/rezepte/api/) · [Contributing](https://s-frei.github.io/rezepte/contributing/add-a-language/)

</div>

Rezepte keeps a household's recipes in one place. Everyone in the house gets a login, writes recipes with ingredient groups and steps, tags and searches them, and cooks from a phone at the stove. It runs as **one binary** with the web app built in, an SQLite database and your photos in a single data directory — no Node runtime, no database server, no external services.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/readme/overview-desktop-dark.png">
  <img alt="The Rezepte recipe overview with search, tag chips and the card grid" src="assets/readme/overview-desktop-light.png">
</picture>

> [!NOTE]
> The interface is English and German, and every member picks their own. Missing your language? [Adding one](https://s-frei.github.io/rezepte/contributing/add-a-language/) takes two files and no programming.

## Why this exists

I needed this myself and found nothing I liked, so I made it. It is also meant to be the more modern option: scoped [API tokens](https://s-frei.github.io/rezepte/api/tokens/) are already there, and the [roadmap](https://s-frei.github.io/rezepte/roadmap/) builds an MCP server for AI assistants and syncing between instances on top of them.

## Try it

Demo mode fills an empty instance with twelve sample recipes, most of them with photos, so you can click through the real app without setting anything up. The photos are AI-generated:

```bash
docker run --rm -p 8060:8060 ghcr.io/s-frei/rezepte:latest --demo
```

Open <http://localhost:8060> and log in as **`demo`** / **`demo1234`**.

Nothing is kept: there is no volume and `--rm` removes the container, so stopping it takes the demo with it. To keep what you write, run your own instance below.

## Features

- **Recipes** — ingredient groups with per-row notes, steps, servings, prep and cooking times and a source link. → [Docs](https://s-frei.github.io/rezepte/guide/recipes/)
- **Photos** — upload several per recipe, reorder them, pick the cover, open them in a lightbox. → [Docs](https://s-frei.github.io/rezepte/guide/images/)
- **Tags and search** — find recipes by title, description, ingredient or tag, and narrow by cooking time or favorites. → [Docs](https://s-frei.github.io/rezepte/guide/tags-and-search/)
- **Favorites** — a personal star per member; the household shares one collection, the stars stay private. → [Docs](https://s-frei.github.io/rezepte/guide/tags-and-search/)
- **Servings** — scale every quantity to the number of people at the table. → [Docs](https://s-frei.github.io/rezepte/guide/servings/)
- **Cook mode** — a full-screen, step-by-step view made for a phone propped up next to the stove. → [Docs](https://s-frei.github.io/rezepte/guide/cook-mode/)
- **Command palette** — <kbd>⌘</kbd><kbd>K</kbd> / <kbd>Ctrl</kbd><kbd>K</kbd> jumps to a recipe by name, or opens the editor and the settings, without leaving the keyboard. → [Docs](https://s-frei.github.io/rezepte/guide/shortcuts/)
- **Members** — accounts, roles and password resets for everyone in the household. → [Docs](https://s-frei.github.io/rezepte/guide/users/)
- **Light and dark** — follows the system setting, or pick one. → [Docs](https://s-frei.github.io/rezepte/guide/settings/)
- **English and German** — each member picks their own interface language, and it follows them to every device. → [Docs](https://s-frei.github.io/rezepte/guide/settings/)
- **HTTP API** — everything the app does, reachable with a scoped bearer token for scripts and other instances, documented with a generated OpenAPI reference. → [Docs](https://s-frei.github.io/rezepte/api/)

## Run your own instance

The same image as the demo, but with a volume for the database and the photos and an owner password of your own, so everything survives a restart:

```bash
docker run -d --name rezepte -p 8060:8060 \
  -e REZEPTE_ADMIN_PASSWORD=change-me-now \
  -v rezepte-data:/data \
  ghcr.io/s-frei/rezepte:latest
```

Open <http://localhost:8060> and log in as **`admin`** with the password you set. That account owns the instance; there is no self-registration, so add everyone else from the settings once you are in.

> [!IMPORTANT]
> `REZEPTE_ADMIN_PASSWORD` is read only on the very first start, to create the owner. Afterwards change the password inside the app — editing the variable does nothing.

Compose is the setup Rezepte is meant to be run with. See [Getting started](https://s-frei.github.io/rezepte/getting-started/) for the compose file, systemd, binaries and every setting.

## Documentation

Everything lives at **[s-frei.github.io/rezepte](https://s-frei.github.io/rezepte/)**:

| Page | What you find there |
| --- | --- |
| [Getting started](https://s-frei.github.io/rezepte/getting-started/) | Run Rezepte with Docker or as a binary, and log in for the first time. |
| [Tour of the app](https://s-frei.github.io/rezepte/guide/) | Every feature, with screenshots. |
| [Operations](https://s-frei.github.io/rezepte/operations/data-and-backup/) | Where the data lives, backups, updates and reverse proxies. |
| [API](https://s-frei.github.io/rezepte/api/) | The HTTP API behind the app, with a generated reference. |
| [Contributing](https://s-frei.github.io/rezepte/contributing/add-a-language/) | Translate the interface into another language. |

The site serves itself to AI assistants as well, generated from the same pages so it cannot drift from them: [`/llms.txt`](https://s-frei.github.io/rezepte/llms.txt) indexes the manual, [`/llms-full.txt`](https://s-frei.github.io/rezepte/llms-full.txt) is all of it in one file, and `/llms.mdx/<path>/content.md` is any single page as raw Markdown.

## Development

The stack is a Go service serving a bundled SvelteKit SPA, with SQLite for storage. Every tool is pinned and every task runs through [mise](https://mise.jdx.dev):

```bash
mise trust && mise install && mise run setup   # tools and dependencies
mise run dev                                   # app on :9060, API on :8060
mise run check                                 # every lint, format and test gate
```

`mise run dev` creates a local `admin` / `admin1234` login on first start and prints it.

The long-form developer documentation — architecture, conventions and how-tos — lives in the repository under [`docs/memory/content/`](docs/memory/content/) and is not published.

## Contributing

Contributions are welcome. Open an issue for anything you hit, and for a pull request: branch off `develop`, keep `mise run check` green, and write everything in English except the UI copy in `frontend/messages/`, one catalog per language.

A translation is the easiest way in: [Add a language](https://s-frei.github.io/rezepte/contributing/add-a-language/) walks through it, and it needs no code.

> [!NOTE]
> **Rezepte is developed AI-driven.** Most of the code, and all of the documentation, is written by AI coding agents working against the guidelines and the agent memory in this repository, reviewed and steered by me. It is an experiment as much as a workflow, and a good part of why I enjoy building this — I do it for fun. Stated here so nobody has to guess.

## License

[Apache License 2.0](LICENSE)
