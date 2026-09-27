# Contributing to Rezepte

Thanks for wanting to help. Rezepte is a small project with one maintainer, so the right channel makes the difference between a quick answer and a long wait.

## Where things go

| You have | Go to |
|---|---|
| A bug: something does not work as the [documentation](https://s-frei.github.io/rezepte/) says | [Open an issue](https://github.com/s-frei/rezepte/issues/new) with the version (`rezepte --version`), what you did, what you expected and what happened instead |
| An idea or a feature wish | [Ideas in Discussions](https://github.com/s-frei/rezepte/discussions/categories/ideas) |
| A question about installing or using Rezepte | [Q&A in Discussions](https://github.com/s-frei/rezepte/discussions/categories/q-a) |
| A security problem | **Not an issue.** See [SECURITY.md](SECURITY.md) |
| A new interface language | [Add a language](https://s-frei.github.io/rezepte/contributing/add-a-language/) - a pull request is welcome right away |

## Pull requests

Translations can come as a pull request without asking first. For anything else, start with an issue or a discussion, so we agree on the change before you spend time on it - a pull request that arrives unannounced may not fit where Rezepte is going.

When you do open one:

- Base it on `develop`, not `main`. `main` only moves with releases.
- Install the toolchain with [mise](https://mise.jdx.dev): `mise trust && mise install && mise run setup`. Every tool comes from there.
- `mise run check` must pass. It is the same gate CI runs.
- Read [`AGENTS.md`](AGENTS.md) first. It lists the rules the codebase follows - American English everywhere, UI texts in both `frontend/messages/en.json` and `de.json`, commit message format - and links the developer notes in [`docs/memory/content/`](docs/memory/content/index.mdx), which explain how the parts fit together and why.

By contributing you agree that your contribution is licensed under the [Apache License 2.0](LICENSE), like the rest of Rezepte.
