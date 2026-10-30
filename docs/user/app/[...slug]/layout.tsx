import { DocsLayout } from 'fumadocs-ui/layouts/notebook';
import type * as PageTree from 'fumadocs-core/page-tree';
import type { ReactNode } from 'react';
import { NewBadge, isNewIn } from '@/components/new-in';
import { ReleaseNavName, isReleaseUrl } from '@/components/releases';
import { baseOptions } from '@/lib/layout.shared';
import { source } from '@/lib/source';

// The sidebar shows a release as its version, large, with the release's name
// beneath, instead of the whole "v1.3.0 — …" title in sidebar type; and it
// marks a page whose `new` names the newest release, so the mark leaves by
// itself with the next one.
function sidebarTree<T extends PageTree.Root | PageTree.Folder>(node: T): T {
	return {
		...node,
		children: node.children.map((child) => {
			if (child.type === 'folder') return sidebarTree(child);
			if (child.type !== 'page') return child;
			if (isReleaseUrl(child.url) && typeof child.name === 'string') return { ...child, name: <ReleaseNavName title={child.name} /> };
			if (isNewIn(source.getNodePage(child)?.data.new))
				return {
					...child,
					name: (
						<>
							{child.name}
							<NewBadge className="ms-auto" />
						</>
					)
				};
			return child;
		})
	};
}

// The notebook layout puts the sections - the root folders of content/ - as
// tabs into the navbar, so the sidebar lists only the open one.
export default function Layout({ children }: { children: ReactNode }) {
	return (
		<DocsLayout
			tree={sidebarTree(source.getPageTree())}
			tabMode="navbar"
			// The navbar tabs render only the title, so the section's icon goes in
			// with it. On phones the tabs become the sidebar's dropdown, which shows
			// the icon on its own, hence hidden there.
			tabs={{
				transform: (option, node) => ({
					...option,
					title: (
						<span className="inline-flex items-center gap-1.5 [&_svg]:size-4">
							<span className="contents max-md:hidden">{node.icon}</span>
							{option.title}
						</span>
					)
				})
			}}
			{...baseOptions()}
		>
			{children}
		</DocsLayout>
	);
}
