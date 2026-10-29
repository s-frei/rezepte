// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		interface PageState {
			// The name typed on /login, carried to /forgot-password without
			// putting it in the URL.
			login?: string;
			// A recipe opened from the overview: its back button steps back
			// to that overview, filters and place in the list included.
			fromOverview?: boolean;
		}
		// interface Platform {}
	}
}

export {};
