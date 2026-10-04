import { expect, type Page, type TestInfo } from "@playwright/test";

import { DEMO_USER } from "../scripts/demo-user";

// RZP_DEMO_NOW in ../mise.toml: the demo dates its samples, comments and
// tokens from the same instant, so the days it shows hold in every run. With
// an offset, so the instant does not depend on the machine's time zone.
export const FIXED_TIME = new Date("2026-09-17T10:30:00+02:00");

/** True for the Pixel 7 projects. */
export function isMobile(testInfo: TestInfo): boolean {
  return testInfo.project.name.startsWith("mobile");
}

/**
 * Fixes the clock, keeps the comments unseen, stores the theme
 * matching the project's color scheme (the app reads `rezepte-theme` before
 * first paint and sets `data-theme` on <html>, see frontend/src/app.html)
 * and optionally logs in as the demo admin.
 */
export async function prepare(
  page: Page,
  testInfo: TestInfo,
  opts: { login?: boolean } = {},
): Promise<void> {
  const theme = testInfo.project.name.endsWith("-dark") ? "dark" : "light";
  await page.clock.setFixedTime(FIXED_TIME);
  // Opening a recipe marks its comments seen, which takes the dot off
  // its card. The specs share one demo instance and run in a fixed order, so
  // a detail shot would decide whether a later overview shot has its dots;
  // answering the call here keeps every picture as the demo seeds it.
  await page.route("**/api/v1/recipes/*/comments/seen", (route) =>
    route.fulfill({ status: 204 }),
  );
  await page.addInitScript(
    (t) => window.localStorage.setItem("rezepte-theme", t),
    theme,
  );
  if (opts.login) {
    await page.goto("/login");
    await page.getByLabel("Username").fill(DEMO_USER.username);
    await page.getByLabel("Password").fill(DEMO_USER.password);
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page).toHaveURL("/");
  }
}

/** Waits for fonts, network and running animations, then takes the named screenshot (the name is the contract with <Screenshot name=…/>). */
export async function shot(page: Page, name: string): Promise<void> {
  await page.evaluate(() => document.fonts.ready);
  await page.waitForLoadState("networkidle");
  // Svelte's transitions are Web Animations, and a dialog caught mid-scale
  // (200ms from 0.95) differs from its baseline by a third of the image -
  // which is what made the command palette flake. Racing a deadline keeps a
  // never-settling animation from turning that into a 30s timeout instead.
  await page.evaluate(
    () =>
      Promise.race([
        Promise.allSettled(document.getAnimations().map((a) => a.finished)),
        new Promise((resolve) => setTimeout(resolve, 1500)),
      ]) as Promise<unknown>,
  );
  await expect(page).toHaveScreenshot(`${name}.png`);
}

/**
 * The password of every user these specs bootstrap: the username with `1234`
 * appended, the repository's rule for development credentials (see
 * docs/memory/content/conventions/dev-credentials.mdx). POST /api/v1/users
 * requires eight characters, so such a username needs at least four.
 */
export function devPassword(username: string): string {
  return `${username}1234`;
}

/** Creates a user through the API when it does not exist yet (409 = already there). */
export async function ensureUser(
  page: Page,
  username: string,
  role: "admin" | "user",
): Promise<void> {
  const password = devPassword(username);
  const origin = new URL(page.url()).origin;
  const res = await page.request.post("/api/v1/users", {
    headers: { Origin: origin },
    data: { username, password, role },
  });
  if (!res.ok() && res.status() !== 409) {
    throw new Error(
      `create user ${username}: ${res.status()} ${await res.text()}`,
    );
  }
}

// Mail is configured through the API, on the placeholder server a real
// instance would have: the screenshot instance has none of its own
// (RZP_DEMO_MAIL=off), and the card is the same whether or not it works.
export async function configureMail(page: Page) {
  const response = await page.request.put("/api/v1/settings/mail", {
    headers: { Origin: new URL(page.url()).origin },
    data: {
      host: "smtp.example.org",
      port: 587,
      security: "starttls",
      username: "rezepte@example.org",
      password: "x",
      from: "rezepte@example.org",
      fromName: "Rezepte"
    }
  });
  expect(response.ok()).toBe(true);
}

export async function turnMailOff(page: Page) {
  await page.request.delete("/api/v1/settings/mail", {
    headers: { Origin: new URL(page.url()).origin }
  });
}
