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
	const search = useDocsSearch({ client: staticClient({ from: '/api/search', tag }) }, [tag]);
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
