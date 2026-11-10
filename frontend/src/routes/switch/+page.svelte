<script lang="ts">
	import Pencil from '@lucide/svelte/icons/pencil';
	import X from '@lucide/svelte/icons/x';
	import { flushSync, tick } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { toast } from 'svelte-sonner';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Path } from '$app/types';
	import { resolvePath } from '#lib/paths.js';
	import { markArrival } from '#lib/arrival.js';
	import { wait } from '#lib/page-transition.js';
	import { announceSession } from '#lib/session-broadcast.js';
	import mascot from '$brand/lockups/rezepte-mascot-switch.webp?url';
	import { ApiError } from '#lib/api/client.js';
	import {
		listDeviceAccounts,
		removeDeviceAccount,
		switchAccount,
		type DeviceAccount
	} from '#lib/api/auth.js';
	import AccountCard from '#lib/components/switch/AccountCard.svelte';
	import AddCard from '#lib/components/switch/AddCard.svelte';
	import HelloSplash, { grow } from '#lib/components/switch/HelloSplash.svelte';
	import RemoveDialog from '#lib/components/switch/RemoveDialog.svelte';
	import { seenLabel } from '#lib/components/switch/switch.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let accounts = $derived(data.accounts);
	let editing = $state(false);
	let removing = $state<DeviceAccount | null>(null);
	let removeOpen = $state(false);
	let pending = $state<string | null>(null);
	let chosen = $state<DeviceAccount | null>(null);
	// The splash holds the card's view-transition name only while it grows.
	let morphing = $state(false);
	let heading = $state<HTMLElement | null>(null);
	// Leaving folds the list back into its pile; picking lets the other cards fall away.
	let leaving = $state(false);
	let picking = $state(false);
	// The greeting fades before the reload; its ground stays for the arrival.
	let greeted = $state(false);

	const now = new Date();
	const locale = getLocale();
	// Nothing to remove, nothing to edit: the toggle goes, and editing ends
	// with the last card removed.
	const canEdit = $derived(accounts.length > 0);
	const isEditing = $derived(editing && canEdit);

	async function close() {
		if (leaving) return;
		leaving = true;
		if (!prefersReducedMotion.current) await wait(350);
		if (history.length > 1) history.back();
		else void goto(resolve('/'));
	}

	// The login in add mode, carrying `next` so a deep link still lands
	// after signing in again (`/` is left out, it is the login's default).
	function addLogin(username?: string) {
		const query = {
			add: '1',
			...(username && { username }),
			...(data.next !== '/' && { next: data.next })
		};
		return resolve(`login?${new URLSearchParams(query)}`);
	}

	function signInAgain(account: DeviceAccount) {
		void goto(addLogin(account.username));
	}

	// A remove under way takes no second one: a double tap on the dialog's
	// Remove lands on the closing dialog, then on the card behind it.
	let busy = false;

	function askRemove(account: DeviceAccount) {
		if (busy) return;
		removing = account;
		removeOpen = true;
	}

	// The dialog hands no focus back on a confirmed remove (its trigger, the
	// card's ×, is about to go): the next card takes it, else the one
	// before, else the heading.
	async function remove() {
		const account = removing;
		if (!account || busy) return;
		busy = true;
		const at = accounts.findIndex((a) => a.id === account.id);
		try {
			await removeDeviceAccount(account.id);
			accounts = accounts.filter((a) => a.id !== account.id);
		} catch {
			toast.error(m.switch_error());
			focusCard(account.id);
			return;
		} finally {
			busy = false;
		}
		await tick();
		const next = accounts[at] ?? accounts[at - 1];
		if (next) focusCard(next.id);
		else heading?.focus();
	}

	function focusCard(id: string) {
		document
			.querySelector<HTMLElement>(`[data-account="${CSS.escape(id)}"] button:last-of-type`)
			?.focus();
	}

	// The picked card grows into the splash (a view transition); the switch
	// itself runs meanwhile, and the app reloads as the new person on `next`,
	// so nothing of the one before stays in memory.
	async function pick(account: DeviceAccount) {
		if (pending) return;
		const switched = switchAccount(account.id);
		switched.catch(() => {});
		try {
			await splash(account);
			await Promise.all([switched, wait(900)]);
			announceSession(account.id);
			// The app paints the splash's ground again over the reload and
			// closes it into the avatar (#lib/arrival.ts).
			if (!prefersReducedMotion.current) {
				greeted = true;
				await wait(150);
			}
			const target = resolvePath(data.next as Path);
			await markArrival(account.color, target);
			location.assign(target);
		} catch (error) {
			chosen = null;
			pending = null;
			picking = false;
			if (error instanceof ApiError && error.status === 409) signInAgain(account);
			else if (error instanceof ApiError && error.status === 404) await reload();
			else toast.error(m.switch_error());
		}
	}

	async function reload() {
		try {
			accounts = await listDeviceAccounts();
		} catch {
			toast.error(m.switch_error());
		}
	}

	async function splash(account: DeviceAccount) {
		flushSync(() => (pending = account.id));
		morphing = true;
		await grow(() => {
			flushSync(() => (chosen = account));
			// The new image is live: the rest of the list falls away under the growing card.
			flushSync(() => (picking = true));
		});
		morphing = false;
	}
</script>

<svelte:head>
	<title>{m.switch_title()} · {m.app_name()}</title>
</svelte:head>

<!-- Inert behind the splash: the page underneath takes no keys. -->
<main
	class="flex min-h-dvh flex-col bg-background md:justify-center-safe"
	class:leaving
	class:picking
	inert={!!pending}
>
	<div class="mx-auto flex w-full max-w-[560px] flex-col px-5 pt-5 pb-12 md:pt-8">
		<header class="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-4">
			<img
				src={mascot}
				alt=""
				class="mascot size-14 object-contain min-[360px]:size-16 md:size-20"
			/>
			<!-- The close shares the heading's row, so it centers on its first line. -->
			<div class="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4">
				<h1
					bind:this={heading}
					tabindex="-1"
					class="font-display text-heading-lg font-medium outline-none min-[360px]:text-display-sm md:text-display-md"
				>
					{m.switch_title()}
				</h1>
				<!-- With nobody active there is nothing to go back to: back would
				     only bounce here again. -->
				{#if data.user}
					<button
						type="button"
						aria-label={m.common_close()}
						onclick={close}
						class="press flex size-11 items-center justify-center rounded-full border border-border bg-surface text-text shadow-card transition hover:border-primary hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<X class="size-5" strokeWidth={2.25} aria-hidden="true" />
					</button>
				{/if}
				<p class="col-start-1 mt-0.5 text-body-sm text-balance text-text-muted">
					{isEditing ? m.switch_lead_editing() : m.switch_lead()}
				</p>
			</div>
		</header>

		<div class="mt-7 mb-3 flex min-h-10 items-center justify-between gap-3 md:mt-9">
			<p class="text-body-sm font-medium text-text-muted">{m.switch_on_device()}</p>
			{#if canEdit}
				<button
					type="button"
					onclick={() => (editing = !editing)}
					aria-pressed={isEditing}
					class="press flex h-10 shrink-0 items-center gap-2 rounded-pill px-4 text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {isEditing
						? 'bg-primary text-primary-foreground shadow-cta'
						: 'border border-border bg-surface text-text hover:border-primary hover:text-primary'}"
				>
					{#if isEditing}
						{m.switch_done()}
					{:else}
						<Pencil class="size-4" aria-hidden="true" />
						{m.switch_edit()}
					{/if}
				</button>
			{/if}
		</div>

		<!-- isolate: the pile's z-indexes stay inside the list, under dialogs. -->
		<ul
			class="isolate flex flex-col gap-3"
			style:--n={accounts.length + (data.user ? 1 : 0) + (isEditing ? 0 : 1)}
		>
			{#if data.user}
				<li class="slot relative" style="--i: 0; z-index: 100">
					<AccountCard
						person={data.user}
						current
						editing={isEditing}
						onpick={() => !isEditing && close()}
					/>
				</li>
			{/if}
			{#each accounts as account, n (account.id)}
				{@const i = n + (data.user ? 1 : 0)}
				<li class="slot relative" style="--i: {i}; z-index: {99 - i}" data-account={account.id}>
					<AccountCard
						person={account}
						expired={account.expired}
						seen={seenLabel(account.lastSeenAt, now, locale)}
						editing={isEditing}
						chosen={pending === account.id && !chosen}
						onpick={() =>
							isEditing
								? askRemove(account)
								: account.expired
									? signInAgain(account)
									: pick(account)}
						onremove={() => askRemove(account)}
					/>
				</li>
			{/each}
			{#if !isEditing}
				<li class="slot relative" style="--i: {accounts.length + (data.user ? 1 : 0)}; z-index: 0">
					<AddCard href={addLogin()} />
				</li>
			{/if}
		</ul>
	</div>
</main>

{#if chosen}
	<HelloSplash person={chosen} named={morphing} {greeted} />
{/if}

<RemoveDialog bind:open={removeOpen} person={removing} onconfirm={remove} />

<style>
	@media (prefers-reduced-motion: no-preference) {
		/* The cards start as a pile and fan out into the list with a spring. */
		.slot {
			animation: fan 0.9s
				linear(
					0,
					0.18 7%,
					0.6 17%,
					0.95 27%,
					1.08 35%,
					1.1 40%,
					1.06 49%,
					0.99 61%,
					0.98 70%,
					1 85%,
					1
				)
				both;
			animation-delay: calc(120ms + var(--i, 0) * 45ms);
		}
		.mascot {
			/* `pop` is app.css's. */
			animation: pop 0.7s cubic-bezier(0.3, 1.5, 0.5, 1) both;
		}
		/* A hovered or focused card rises above the pile's order. */
		.slot:hover,
		.slot:has(:global(:focus-visible)) {
			z-index: 101 !important;
		}
		/* Leaving without a pick: the fan-in backwards and faster, last card first. */
		.leaving .slot {
			/* Not `fan` reversed: that holds each card opaque until it reaches
			   the pile, where this one fades on the way. */
			animation: gather 0.22s ease-in both;
			animation-delay: calc((var(--n, 1) - 1 - var(--i, 0)) * 40ms);
		}
		.leaving .mascot {
			animation: pop 0.25s ease-out reverse both;
		}
		.picking .slot {
			animation: fall 0.2s ease-out both;
		}
		.press:active {
			scale: 0.96;
		}
	}
	@keyframes fan {
		from {
			translate: 0 calc(var(--i, 0) * -92px);
			scale: calc(1 - var(--i, 0) * 0.025);
			opacity: 0;
		}
		12% {
			opacity: 1;
		}
	}
	@keyframes gather {
		to {
			translate: 0 calc(var(--i, 0) * -92px);
			scale: calc(1 - var(--i, 0) * 0.025);
			opacity: 0;
		}
	}
	@keyframes fall {
		to {
			translate: 0 24px;
			opacity: 0;
		}
	}
</style>
