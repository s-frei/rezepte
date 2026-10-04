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
		}
		// interface Platform {}
	}
}

export {};
