import { expect, type Page, type TestInfo } from "@playwright/test";

export const FIXED_TIME = new Date("2026-09-17T10:30:00");
export const DEMO_USER = { username: "demo", password: "demo1234" };

/** True for the Pixel 7 projects. */
export function isMobile(testInfo: TestInfo): boolean {
  return testInfo.project.name.startsWith("mobile");
}

/**
 * Fixes the clock, stores the theme matching the project's colour scheme
 * (the app reads `rezepte-theme` before first paint and sets `data-theme` on
 * <html>, see frontend/src/app.html) and optionally logs in as the demo
 * admin.
 */
export async function prepare(
  page: Page,
  testInfo: TestInfo,
  opts: { login?: boolean } = {},
): Promise<void> {
  const theme = testInfo.project.name.endsWith("-dark") ? "dark" : "light";
  await page.clock.setFixedTime(FIXED_TIME);
  await page.addInitScript(
    (t) => window.localStorage.setItem("rezepte-theme", t),
    theme,
  );
  if (opts.login) {
    await page.goto("/login");
    await page.getByLabel("Benutzername").fill(DEMO_USER.username);
    await page.getByLabel("Passwort").fill(DEMO_USER.password);
    await page.getByRole("button", { name: "Anmelden" }).click();
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

/** Creates a user through the API when it does not exist yet (409 = already there). */
export async function ensureUser(
  page: Page,
  username: string,
  password: string,
  role: "admin" | "user",
): Promise<void> {
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
