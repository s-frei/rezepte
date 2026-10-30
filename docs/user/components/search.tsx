'use client';
import { useDocsSearch } from 'fumadocs-core/search/client';
import { staticClient } from 'fumadocs-core/search/client/orama-static';
import {
	SearchDialog as Dialog,
	SearchDialogClose,
	SearchDialogContent,
	SearchDialogFooter,
	SearchDialogHeader,
	SearchDialogIcon,
	SearchDialogInput,
	SearchDialogList,
	SearchDialogOverlay,
	type SharedProps,
	TagsList,
	TagsListItem
} from 'fumadocs-ui/components/dialog/search';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { SECTIONS, sectionOf } from '@/lib/sections';

// The static search client's default endpoint resolves via `import.meta.env.BASE_URL`,
// which Turbopack (Next 16's default builder, used here) polyfills per module from
// `basePath` - so it would in fact resolve correctly on its own. Pass the endpoint
// explicitly via `from` anyway, so it does not depend on the builder polyfilling
// `import.meta.env` - Turbopack does, but a `--webpack` build or another builder might
// not. Do not read this as "simplify away, Next resolves it" - the point is not to
// depend on that.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

/**
 * The search dialog, filtered to the section the reader is in; clicking the
 * active section again searches everything. Assembled from Fumadocs' parts
 * rather than its DefaultSearchDialog, which renders the tag filter outside
 * the popup - below the page instead of in the dialog.
 */
export function SearchDialog(props: SharedProps) {
	const here = sectionOf(usePathname().replace(/\/$/, '') || '/');
	const [tag, setTag] = useState(here);
	// Each opening starts from the section the reader is in now.
	const [opened, setOpened] = useState(props.open);
	if (props.open !== opened) {
		setOpened(props.open);
		if (props.open) setTag(here);
	}
	const search = useDocsSearch({ client: staticClient({ from: `${basePath}/api/search`, tag }) }, [tag]);
	return (
		<Dialog {...search} {...props}>
			<SearchDialogOverlay />
			<SearchDialogContent>
				<SearchDialogHeader>
					<SearchDialogIcon />
					<SearchDialogInput />
					<SearchDialogClose />
				</SearchDialogHeader>
				<SearchDialogList />
				<SearchDialogFooter>
					<TagsList tag={tag} onTagChange={setTag} allowClear>
						{SECTIONS.map(({ name, value }) => (
							<TagsListItem key={value} value={value}>
								{name}
							</TagsListItem>
						))}
					</TagsList>
				</SearchDialogFooter>
			</SearchDialogContent>
		</Dialog>
	);
}
