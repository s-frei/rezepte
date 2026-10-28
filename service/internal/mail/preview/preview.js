// Language and theme live in the hash (#lang=de&theme=dark) and travel with
// every link to another mail: a browser may keep localStorage apart per
// file:// page, so the hash is what carries them; localStorage only
// remembers them for a page opened without one.
const root = document.documentElement;
const themes = document.querySelectorAll("[data-theme-set]");
const langs = [...document.querySelectorAll("[data-lang-set]")].map((b) => b.dataset.langSet);
let lang, theme;

const read = () => {
  const hash = new URLSearchParams(location.hash.slice(1));
  let stored = {};
  try { stored = { lang: localStorage.getItem("lang"), theme: localStorage.getItem("theme") }; } catch {}
  const l = hash.get("lang") ?? stored.lang, t = hash.get("theme") ?? stored.theme;
  lang = langs.includes(l) ? l : langs[0];
  theme = t === "light" || t === "dark" ? t : "";
};

const show = () => {
  const hash = new URLSearchParams(theme ? { lang, theme } : { lang }).toString();
  history.replaceState(null, "", "#" + hash);
  try { localStorage.setItem("lang", lang); localStorage.setItem("theme", theme); } catch {}
  if (theme) root.dataset.theme = theme; else delete root.dataset.theme;
  themes.forEach((b) => b.setAttribute("aria-pressed", String(theme === b.dataset.themeSet)));
  document.querySelectorAll("[data-lang-set]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.langSet === lang)));
  document.querySelectorAll("[data-lang]").forEach((el) => { el.hidden = el.dataset.lang !== lang; });
  document.querySelectorAll('a[href*="index.html"]').forEach((a) => { a.hash = hash; });
  // System keeps the letter's own media query; light and dark load a copy that forces it.
  document.querySelectorAll("iframe[data-mail]").forEach((f) => {
    const src = f.dataset.mail.replace("{lang}", lang) + (theme ? "." + theme : "") + ".html";
    if (f.getAttribute("src") !== src) f.setAttribute("src", src);
  });
};

themes.forEach((b) => b.addEventListener("click", () => { theme = b.dataset.themeSet; show(); }));
document.querySelectorAll("[data-lang-set]").forEach((b) => b.addEventListener("click", () => { lang = b.dataset.langSet; show(); }));
addEventListener("hashchange", () => { read(); show(); });
read();
show();

// The overview: find mails by name or subject and by the role they reach.
const q = document.getElementById("q");
if (q) {
  const roleBox = document.getElementById("roles");
  const roleButtons = roleBox.querySelectorAll("[data-role]");
  const filter = () => {
    const words = q.value.trim().toLowerCase().split(/\s+/).filter(Boolean);
    const picked = [...roleButtons].filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.dataset.role);
    roleBox.classList.toggle("any-role", picked.length === 0);
    document.getElementById("clear").hidden = picked.length === 0 && words.length === 0;
    let shown = 0;
    document.querySelectorAll(".kind").forEach((kind) => {
      let inKind = 0;
      kind.querySelectorAll(".template").forEach((t) => {
        const roles = t.dataset.roles.split(" ");
        const fits = picked.length === 0 || picked.some((r) => roles.includes(r));
        let inTemplate = 0;
        t.querySelectorAll(".card").forEach((c) => {
          const text = c.dataset.find.toLowerCase();
          c.hidden = !fits || !words.every((w) => text.includes(w));
          if (!c.hidden) inTemplate++;
        });
        t.hidden = inTemplate === 0;
        inKind += inTemplate;
      });
      kind.hidden = inKind === 0;
      shown += inKind;
    });
    document.getElementById("empty").hidden = shown > 0;
    // The jump bar counts what the filters leave.
    document.querySelectorAll("[data-count]").forEach((el) => {
      const kind = document.getElementById(el.dataset.count);
      el.textContent = kind.querySelectorAll(".card:not([hidden])").length;
      el.closest("button").disabled = kind.hidden;
    });
    dispatchEvent(new Event("scroll")); // re-mark the kind in view
  };
  const reset = () => { q.value = ""; roleButtons.forEach((b) => b.setAttribute("aria-pressed", "false")); filter(); };
  roleButtons.forEach((b) => b.addEventListener("click", () => {
    b.setAttribute("aria-pressed", String(b.getAttribute("aria-pressed") !== "true"));
    filter();
  }));
  q.addEventListener("input", filter);
  q.addEventListener("keydown", (e) => { if (e.key === "Escape") { reset(); q.blur(); } });
  document.getElementById("clear").addEventListener("click", reset);
  document.getElementById("reset").addEventListener("click", reset);
  addEventListener("keydown", (e) => {
    if (e.key === "/" && document.activeElement !== q) { e.preventDefault(); q.focus(); }
  });
}

// The jump bar scrolls to a kind of mail and marks the one in view. It
// scrolls rather than links, since the hash holds language and theme.
const jumps = document.querySelectorAll("[data-jump]");
if (jumps.length) {
  const smooth = !matchMedia("(prefers-reduced-motion: reduce)").matches;
  jumps.forEach((b) => b.addEventListener("click", () => {
    document.getElementById(b.dataset.jump).scrollIntoView({ behavior: smooth ? "smooth" : "auto" });
  }));
  const mark = () => {
    // The last kind whose top passed the line under the header is in view.
    const line = document.querySelector("header.bar").getBoundingClientRect().bottom + 24;
    let current = null;
    document.querySelectorAll(".kind:not([hidden])").forEach((k) => { if (k.getBoundingClientRect().top <= line) current = k.id; });
    if (innerHeight + scrollY >= document.documentElement.scrollHeight - 2) {
      const all = document.querySelectorAll(".kind:not([hidden])");
      current = all.length ? all[all.length - 1].id : current;
    }
    current ??= document.querySelector(".kind:not([hidden])")?.id;
    jumps.forEach((b) => b.setAttribute("aria-current", String(b.dataset.jump === current)));
  };
  addEventListener("scroll", mark, { passive: true });
  addEventListener("resize", mark);
  mark();
}

// The arrow keys step through the mails, outside a text field.
addEventListener("keydown", (e) => {
  if (e.metaKey || e.ctrlKey || e.altKey || e.target.closest("input, textarea")) return;
  const rel = { ArrowLeft: "prev", ArrowRight: "next" }[e.key];
  const to = rel && document.querySelector(`a[rel=${rel}]`);
  if (to) location.href = to.href;
});
