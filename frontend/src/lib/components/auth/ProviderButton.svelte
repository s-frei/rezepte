<script lang="ts">
	import Button from '$lib/components/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		intent,
		name,
		next = '',
		setup = ''
	}: {
		intent: 'login' | 'link' | 'setup';
		name: string;
		/** Where a login continues to; the service checks it again. */
		next?: string;
		/** The setup token, for intent "setup". */
		setup?: string;
	} = $props();
</script>

<!-- A real form post, not fetch: the browser has to follow the service's
     303 to the provider. Same-origin, so it passes the Origin check, and the
     setup token rides in the body rather than in a URL. -->
<form method="POST" action="/api/v1/auth/oidc/start">
	<input type="hidden" name="intent" value={intent} />
	<input type="hidden" name="next" value={next} />
	<input type="hidden" name="setup" value={setup} />
	<Button type="submit" variant="secondary" size="lg" class="w-full">
		{m.oidc_continue({ name })}
	</Button>
</form>
