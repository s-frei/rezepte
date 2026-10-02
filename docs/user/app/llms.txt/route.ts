import { absoluteLinks, docsLlms } from '@/lib/source';

export const revalidate = false;

export async function GET() {
	return new Response(absoluteLinks(await docsLlms.index()), { headers: { 'Content-Type': 'text/plain' } });
}
