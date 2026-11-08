<script lang="ts">
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { Dialog } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { untrack } from 'svelte';
	import { MediaQuery, SvelteSet } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { isSignedOut } from '#lib/api/client.js';
	import type { IngredientGroup } from '#lib/api/recipes.js';
	import {
		addRecipeToList,
		createShoppingList,
		listShoppingLists,
		type ShoppingSummary
	} from '#lib/api/shopping.js';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import Checkbox from '#lib/components/ui/Checkbox.svelte';
	import Input from '#lib/components/ui/Input.svelte';
	import Select from '#lib/components/ui/Select.svelte';
	import ServingsStepper from '#lib/components/recipe/ServingsStepper.svelte';
	import { isChecked } from '#lib/recipe/checked.svelte.js';
	import { formatQuantityFor } from '#lib/recipe/scale.js';
	import { clampServings } from '#lib/recipe/servings.js';
	import { defaultList, pickKey, pickedRefs, preselected } from '#lib/shopping/add.js';
	import { listHref } from '#lib/shopping/href.js';
	import { load, save } from '#lib/storage.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		open = $bindable(false),
		recipeId,
		groups,
		baseServings,
		servings,
		title
	}: {
		open?: boolean;
		recipeId: string;
		groups: IngredientGroup[];
		baseServings: number;
		/** The reader's servings; the dialog starts there. */
		servings: number;
		/** The recipe's title, shown under the dialog's own. */
		title?: string;
	} = $props();

	const LAST_LIST = 'rezepte:last-list';
	const phone = new MediaQuery('max-width: 767.98px');

	let lists = $state<ShoppingSummary[]>([]);
	let loaded = $state(false);
	let loadFailed = $state(false);
	let listId = $state('');
	let creating = $state(false);
	let newName = $state('');
	let chosen = $state(1);
	let saving = $state(false);
	const picked = new SvelteSet<string>();

	const total = $derived(groups.reduce((sum, g) => sum + g.ingredients.length, 0));
	const count = $derived(picked.size);
	const showNew = $derived(creating || (loaded && !loadFailed && lists.length === 0));
	// The recipe on the picked list already: adding again adds its amounts to the ones there.
	const already = $derived(
		!showNew && !!lists.find((l) => l.id === listId)?.recipes.some((r) => r.recipeId === recipeId)
	);
	const canSubmit = $derived(
		count > 0 && !loadFailed && (showNew ? !!newName.trim() : !!listId) && !saving
	);

	// A fresh start on every opening: the reader's servings, what the recipe
	// page has not ticked off, and the lists as they are now.
	$effect(() => {
		if (!open) return;
		untrack(start);
	});

	function start() {
		chosen = clampServings(servings);
		creating = false;
		newName = '';
		picked.clear();
		for (const key of preselected(recipeId, groups, isChecked)) picked.add(key);
		void loadLists();
	}

	// Openings overlap when the dialog is closed and reopened before an answer
	// arrives; only the newest request's answer counts.
	let req = 0;
	async function loadLists() {
		const mine = ++req;
		loaded = false;
		loadFailed = false;
		try {
			const all = (await listShoppingLists()).items;
			if (mine !== req) return;
			lists = all;
			listId = defaultList(all, load(LAST_LIST));
		} catch (error) {
			if (mine !== req) return;
			loadFailed = true;
			if (!isSignedOut(error)) toast.error(m.lists_load_error());
		} finally {
			if (mine === req) loaded = true;
		}
	}

	function allPicked() {
		return count === total;
	}

	function toggleAll() {
		const all = allPicked();
		picked.clear();
		if (all) return;
		groups.forEach((g, gi) => g.ingredients.forEach((_, i) => picked.add(pickKey(gi, i))));
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!canSubmit) return;
		saving = true;
		try {
			let target = listId;
			let name = lists.find((l) => l.id === listId)?.name ?? '';
			if (showNew) {
				const created = await createShoppingList(newName.trim());
				target = created.id;
				name = created.name;
				// A failed add below must retry on this list, not create a second one.
				lists = [
					{
						id: created.id,
						name: created.name,
						openCount: 0,
						doneCount: 0,
						recipes: [],
						updatedAt: created.updatedAt,
						owner: created.owner,
						members: [],
						updatedBy: null
					},
					...lists
				];
				listId = created.id;
				creating = false;
				newName = '';
			}
			await addRecipeToList(target, {
				recipeId,
				servings: chosen,
				ingredients: pickedRefs(groups, picked)
			});
			save(LAST_LIST, target);
			open = false;
			const n = count;
			toast.success(
				n === 1
					? m.add_to_list_added_one({ list: name })
					: m.add_to_list_added({ count: n, list: name }),
				{ action: { label: m.add_to_list_open(), onClick: () => goto(listHref(target)) } }
			);
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.add_to_list_error());
		} finally {
			saving = false;
		}
	}

	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium">{m.add_to_list_title()}</Dialog.Title>
	{#if title}<p class="mt-1 text-caption text-text-muted">{title}</p>{/if}
	<div class="mt-5 space-y-5">
		<div>
			<div class="mb-2 flex items-baseline justify-between">
				<span class={label}>{m.add_to_list_list_label()}</span>
				{#if lists.length > 0 && !loadFailed}
					<!-- A 44px target that keeps the line's height: padding out, margin back. -->
					<button
						type="button"
						onclick={() => (creating = !creating)}
						class="-mx-2 -mt-4 -mb-2 rounded-md px-2 pt-4 pb-2 text-caption font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						{creating ? m.common_cancel() : m.add_to_list_new_list()}
					</button>
				{/if}
			</div>
			{#if showNew}
				<Input
					id="add-to-list-name"
					label={m.lists_new_name_label()}
					placeholder={m.lists_new_name_placeholder()}
					autocomplete="off"
					maxlength={80}
					bind:value={newName}
				/>
			{:else if loadFailed}
				<button
					type="button"
					onclick={loadLists}
					class="flex min-h-11 w-full items-center justify-between rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-body-sm text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<span class="min-w-0 text-caption leading-tight">{m.lists_load_error()}</span>
					<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
						>{m.add_to_list_retry()}</span
					>
				</button>
			{:else}
				<Select
					bind:value={listId}
					options={lists.map((l) => ({ value: l.id, label: l.name }))}
					label={m.add_to_list_list_label()}
					class="w-full justify-between bg-surface-elevated"
				/>
				{#if already}
					<p class="mt-1.5 text-caption text-text-muted">{m.add_to_list_already()}</p>
				{/if}
			{/if}
		</div>
		<div class="flex items-center justify-between">
			<span class={label}>{m.add_to_list_servings()}</span>
			<ServingsStepper value={chosen} onchange={(next) => (chosen = clampServings(next))} />
		</div>
		<div>
			<div class="mb-2 flex items-baseline justify-between">
				<span class={label}>{m.add_to_list_ingredients({ selected: count, total })}</span>
				<button
					type="button"
					onclick={toggleAll}
					class="-mx-2 -mt-4 -mb-2 rounded-md px-2 pt-4 pb-2 text-caption font-semibold text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					{allPicked() ? m.add_to_list_select_none() : m.add_to_list_select_all()}
				</button>
			</div>
			<!-- The frame is rounded and clips; on a computer the ingredients scroll
			     inside its padding, so the scrollbar runs on the straight part of the
			     edge (as SearchBar's list). -->
			<div class="overflow-hidden rounded-2xl border border-border bg-surface-elevated px-1.5 py-2">
				<div class="px-2.5 md:max-h-[40dvh] md:overflow-y-auto md:overscroll-contain">
					{#each groups as group, g (g)}
						{#if group.name}
							<h3
								class="mt-3 mb-1 font-display text-[16px] font-medium text-primary italic first:mt-1"
							>
								{group.name}
							</h3>
						{/if}
						{#each group.ingredients as ingredient, i (i)}
							{@const key = pickKey(g, i)}
							{@const on = picked.has(key)}
							<label
								class="flex min-h-11 cursor-pointer items-start gap-3 border-b border-dashed border-border py-2.5 text-body last:border-b-0 {on
									? 'text-text'
									: 'text-text-muted'}"
							>
								<span class="mt-[calc((1lh_-_20px)/2)] flex">
									<Checkbox
										bind:checked={() => on, (v) => (v ? picked.add(key) : picked.delete(key))}
										label={ingredient.name}
									/>
								</span>
								<span class="min-w-0 break-words">
									{formatQuantityFor(ingredient.quantity, baseServings, chosen)}
									{ingredient.unit ?? ''}
									{ingredient.name}
								</span>
							</label>
						{/each}
					{/each}
				</div>
			</div>
			<p class="mt-2 text-caption text-text-muted">{m.add_to_list_hint()}</p>
		</div>
	</div>
{/snippet}

{#snippet footer()}
	<Button variant="secondary" class="px-3 max-[340px]:px-2.5 md:px-5" onclick={() => (open = false)}
		>{m.common_cancel()}</Button
	>
	<Button
		type="submit"
		busy={saving}
		disabled={!canSubmit}
		class="min-w-0 grow px-3 whitespace-nowrap md:grow-0 md:px-5"
	>
		<ShoppingBasket class="size-4 shrink-0 max-[340px]:hidden" aria-hidden="true" />
		{count === 1 ? m.add_to_list_submit_one() : m.add_to_list_submit({ count })}
	</Button>
{/snippet}

{#if phone.current}
	<BottomSheet bind:open closeLabel={m.common_cancel()}>
		<form onsubmit={submit} class="flex min-h-0 flex-1 flex-col">
			<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-4">{@render body()}</div>
			<div class="flex gap-2 border-t border-border px-4 pt-3.5 pb-5 min-[340px]:px-5">
				{@render footer()}
			</div>
		</form>
	</BottomSheet>
{:else}
	<BaseDialog bind:open>
		<form onsubmit={submit}>
			{@render body()}
			<DialogActions class="mt-6 border-t border-border pt-4">
				{@render footer()}
			</DialogActions>
		</form>
	</BaseDialog>
{/if}
