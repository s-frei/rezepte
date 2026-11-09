<script lang="ts">
	import ChefHat from '@lucide/svelte/icons/chef-hat';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { userColorClasses } from '#lib/user/color.js';
	import { firstName } from './switch';

	let {
		person,
		named = false,
		greeted = false
	}: {
		person: { id: string; displayName: string; color: string; avatarId: string | null };
		/** Takes over the picked card's view-transition name while it grows. */
		named?: boolean;
		/** Greeted: the contents fade, the ground stays for the app's arrival. */
		greeted?: boolean;
	} = $props();

	const name = $derived(firstName(person.displayName));
</script>

<!-- The picked card, grown into the whole screen (the view transition shares
     its name), greeting the person while the app is set up for them. -->
<div
	role="status"
	class="fixed inset-0 z-60 flex flex-col items-center justify-center gap-5 px-6 text-center {userColorClasses(
		person.color
	)}"
	style:view-transition-name={named ? 'chosen-card' : undefined}
	class:greeted
>
	<span class="mark rounded-full bg-background p-2">
		<PersonMark {person} size="profile" />
	</span>
	<p
		class="hello max-w-full font-display font-medium text-balance hyphens-auto {name.length > 14
			? 'text-heading-lg md:text-display-sm'
			: 'text-display-lg'}"
	>
		{m.switch_hello({ name })}
	</p>
	<p class="sub flex items-center gap-2 text-body opacity-80">
		<ChefHat class="size-4" aria-hidden="true" />
		{m.switch_hello_sub()}
	</p>
</div>

<style>
	@media (prefers-reduced-motion: no-preference) {
		.mark {
			/* `pop` is app.css's. */
			animation: pop 0.6s 0.25s cubic-bezier(0.3, 1.5, 0.5, 1) both;
		}
		.hello {
			animation: rise 0.5s 0.35s cubic-bezier(0.2, 0.8, 0.3, 1) both;
		}
		.sub {
			animation: rise 0.5s 0.5s cubic-bezier(0.2, 0.8, 0.3, 1) both;
		}
		/* Replaces the way in, which holds its end with `both`. */
		.greeted > * {
			animation: away 140ms ease-out both;
		}
	}
	@keyframes away {
		to {
			opacity: 0;
			scale: 0.96;
		}
	}
	@keyframes rise {
		from {
			opacity: 0;
			translate: 0 14px;
		}
	}
</style>
