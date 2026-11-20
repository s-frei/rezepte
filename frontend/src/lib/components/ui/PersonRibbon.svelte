<script lang="ts">
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { userColorClasses } from '#lib/user/color.js';

	let {
		person,
		src,
		text,
		class: extra = ''
	}: {
		person: { displayName: string; color: string; avatarId?: string | null };
		/** The picture's address when it is not the session's `/avatars/...`
		 * (the public page's share-scoped one); `null` shows the initial. */
		src?: string | null;
		/** The sentence with the name in it ("Shared by {name}"), translated
		 * whole so each language keeps its own order. */
		text: (name: string) => string;
		/** The pill's own extras: the recipe page's hover tint. */
		class?: string;
	} = $props();

	// Split where the name goes, so the name alone is set bold (as
	// RecipeColophon does).
	const WHO = '\u0000';
	const parts = $derived(
		text(WHO)
			.split(WHO)
			.flatMap((part, i) => (i > 0 ? [WHO, part] : [part]))
			.filter(Boolean)
	);
	const photo = $derived(src !== undefined ? src !== null : !!person.avatarId);
</script>

<!--
	The profile head's band in miniature: a slim ribbon in the person's color
	with their face on its left edge - their picture when they have one, else
	their initial in their strong tone, which the ribbon's own would swallow.
	The page's ground cuts the face out of the ribbon, as on the author band;
	a photo keeps PersonMark's ring and lines that ring, not the face, up
	with the column. The public page's "Shared by" and the recipe page's
	author ribbon, both under the source line.
-->
<span
	class="inline-flex min-h-8 max-w-full min-w-0 items-center gap-2.5 rounded-pill pr-4 {userColorClasses(
		person.color
	)} {extra}"
>
	<PersonMark
		person={{ id: '', avatarId: null, ...person }}
		{src}
		size="ribbon"
		strong
		class="-my-1.5 ring-3 ring-background {photo ? '' : '-ml-1'}"
	/>
	<span class="min-w-0 py-1.5 text-left text-body-sm wrap-break-word"
		>{#each parts as part, i (i)}{#if part === WHO}<b class="font-semibold">{person.displayName}</b
				>{:else}{part}{/if}{/each}</span
	>
</span>
