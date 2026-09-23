import { DocsBody, DocsDescription, DocsPage, DocsTitle, MarkdownCopyButton, ViewOptionsPopover } from 'fumadocs-ui/layouts/docs/page';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import type { OpenAPIPageProps_Preloaded } from 'fumadocs-openapi/ui';
import { OpenAPIPage } from '@/components/api-page';
import { getMDXComponents } from '@/components/mdx';
import { openapi } from '@/lib/openapi';
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
				<MDX
					components={getMDXComponents({
						a: createRelativeLink(source, page),
						// The body of a generated API page is a single <OpenAPIPage> tag
						// (scripts/generate-api.ts). Preloading here rather than inside the
						// component keeps the document read on the server, where it belongs:
						// lib/openapi.ts must never reach a browser bundle.
						// The generated page passes `document`, `operations` and the
						// `show*` flags; `preloaded` is everything else the component
						// needs, which is why the prop type is the preloaded one
						// minus that key.
						OpenAPIPage: async (props: Omit<OpenAPIPageProps_Preloaded, 'preloaded'>) => (
							<OpenAPIPage {...props} {...(await openapi.preloadOpenAPIPage(page))} />
						)
					})}
				/>
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
