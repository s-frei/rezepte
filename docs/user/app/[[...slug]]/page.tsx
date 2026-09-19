import { DocsBody, DocsDescription, DocsPage, DocsTitle, MarkdownCopyButton, ViewOptionsPopover } from 'fumadocs-ui/layouts/docs/page';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getMDXComponents } from '@/components/mdx';
import { source } from '@/lib/source';
import { getPageMarkdownUrl, OG_SHARED } from '@/lib/shared';

type Props = { params: Promise<{ slug?: string[] }> };

// `/` renders content/index.mdx: with a single collection there is no landing chooser.
export default async function Page({ params }: Props) {
	const { slug = [] } = await params;
	const page = source.getPage(slug);
	if (!page) notFound();

	const MDX = page.data.body;
	return (
		<DocsPage toc={page.data.toc} full={page.data.full}>
			<DocsTitle>{page.data.title}</DocsTitle>
			<DocsDescription>{page.data.description}</DocsDescription>
			<div className="flex flex-row items-center gap-2 border-b pb-6">
				<MarkdownCopyButton markdownUrl={getPageMarkdownUrl(page)} />
				<ViewOptionsPopover
					markdownUrl={getPageMarkdownUrl(page)}
					githubUrl={`https://github.com/s-frei/rezepte/blob/main/docs/user/content/${page.path}`}
				/>
			</div>
			<DocsBody>
				<MDX components={getMDXComponents({ a: createRelativeLink(source, page) })} />
			</DocsBody>
		</DocsPage>
	);
}

export function generateStaticParams() {
	return source.generateParams();
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { slug = [] } = await params;
	const page = source.getPage(slug);
	if (!page) notFound();
	return {
		title: page.data.title,
		description: page.data.description,
		openGraph: { ...OG_SHARED, title: page.data.title, description: page.data.description }
	};
}
