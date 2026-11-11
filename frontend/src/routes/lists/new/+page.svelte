<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import X from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { isSignedOut } from '#lib/api/client.js';
	import { createShoppingList } from '#lib/api/shopping.js';
	import AddItemCard from '#lib/components/shopping/AddItemCard.svelte';
	import ViewSwitch from '#lib/components/shopping/ViewSwitch.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { handKeyboardTo, holdKeyboard } from '#lib/keyboard.js';
	import { listHref } from '#lib/shopping/href.js';
	import { m } from '#lib/paraglide/messages.js';
	import { shell } from '#lib/shell.svelte.js';

	// A list before it exists: its page with the name as a field at the top,
	// where no on-screen keyboard reaches. Only "Create" makes the list;
	// leaving any other way leaves nothing behind.
	let name = $state('');
	let saving = $state(false);
	let field: HTMLInputElement | undefined = $state();

	onMount(() => field && handKeyboardTo(field));

	// The draft is the new list: the bar offers no other one.
	$effect(() => {
		shell.breadcrumb = m.list_breadcrumb({ lists: m.lists_title(), name: m.lists_new() });
		shell.actions = noActions;
		return () => {
			shell.breadcrumb = undefined;
			shell.actions = undefined;
		};
	});

	async function create(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = name.trim();
		if (!trimmed || saving) return;
		// Inside the tap: the list's "What do you need?" takes the keyboard over.
		holdKeyboard();
		saving = true;
		try {
			const list = await createShoppingList(trimmed);
			await goto(listHref(list.id), { replaceState: true, state: { created: true } });
		} catch (error) {
			if (field) handKeyboardTo(field);
			if (!isSignedOut(error)) toast.error(m.lists_create_error());
		} finally {
			saving = false;
		}
	}

	function cancelOnEscape(event: KeyboardEvent) {
		if (event.key === 'Escape') void goto(resolve('/lists'));
	}
</script>

{#snippet noActions()}{/snippet}

<svelte:head><title>{m.lists_new()} · {m.app_name()}</title></svelte:head>

<div class="pt-6 pb-8 md:pt-4">
	<h1 class="sr-only">{m.lists_new()}</h1>
	<!-- The list page's header: phone, close and "Create" above the name; desktop, both beside it. -->
	<form onsubmit={create} class="mb-4">
		<header class="grid grid-cols-[minmax(0,1fr)_auto] items-end gap-x-4">
			<a
				href={resolve('/lists')}
				aria-label={m.common_cancel()}
				class="inline-flex size-10 items-center justify-center justify-self-start rounded-full border border-border bg-surface hover-tint text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:hidden"
			>
				<X class="size-5" aria-hidden="true" />
			</a>
			<div class="col-span-2 row-start-2 mt-5 min-w-0 md:col-span-1 md:row-start-1 md:mt-0">
				<input
					bind:this={field}
					bind:value={name}
					onkeydown={cancelOnEscape}
					aria-label={m.lists_new_name_label()}
					placeholder={m.lists_new_name_placeholder()}
					autocomplete="off"
					enterkeyhint="done"
					maxlength={80}
					class="w-full min-w-0 border-b-2 border-border bg-transparent pb-1 font-display text-display-md font-medium text-text transition-colors outline-none placeholder:text-text-muted focus:border-primary md:text-display-lg"
				/>
			</div>
			<div class="col-start-2 row-start-1 flex items-center gap-2 md:mb-1">
				<Button variant="secondary" href={resolve('/lists')} class="max-md:hidden">
					<X class="size-4" aria-hidden="true" />
					{m.common_cancel()}
				</Button>
				<Button type="submit" busy={saving} disabled={!name.trim()} label={m.lists_new_create()}>
					<Check class="size-4" aria-hidden="true" />
					<span class="md:hidden">{m.lists_new_create_short()}</span>
					<span class="max-md:hidden">{m.lists_new_create()}</span>
				</Button>
			</div>
		</header>
	</form>

	<!-- What the list will be, waiting until it exists. -->
	<div inert class="opacity-40">
		<AddItemCard onadd={async () => false} />
		<div class="mt-4 rounded-2xl bg-surface shadow-card">
			<div
				class="mx-4.5 border-b border-dashed border-border py-3.5 md:mx-6 md:flex md:items-center md:justify-between md:py-3"
			>
				<p class="text-caption text-text-muted max-md:hidden">
					{m.list_counts({ open: 0, done: 0 })}
				</p>
				<ViewSwitch compact />
			</div>
			<div class="px-4.5 pb-3 md:px-6 md:pt-1.5 md:pb-4">
				<div class="py-10 text-center">
					<p class="font-display text-heading font-medium">{m.list_empty_title()}</p>
					<p class="mx-auto mt-1.5 max-w-sm text-body text-text-muted">{m.list_empty_text()}</p>
				</div>
			</div>
		</div>
	</div>
</div>
