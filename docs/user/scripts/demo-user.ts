// The credentials `rezepte --demo` seeds when the operator set no password
// of their own - see service/internal/demo/seed.go, which is where these
// values come from and must stay in step with.
//
// Both build-time tools that drive a demo instance need them: the
// screenshot specs log in through the UI, and fetch-openapi.ts logs in over
// the API because the OpenAPI document is not public
// (docs/memory/content/features/users-and-auth.mdx).
export const DEMO_USER = { username: "demo", password: "demo1234" };
