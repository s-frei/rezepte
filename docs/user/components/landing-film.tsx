'use client';
// The product film on the landing page: public/film/demo-light.mp4, recorded by `mise run //docs/user:film`.
import { useEffect, useRef } from 'react';

/**
 * Muted and looping, it starts once it is in view and pauses out of view, so it costs nothing above the fold. Under reduced
 * motion it never starts by itself: the poster stays and the controls play it. Without JavaScript the controls do the same.
 */
export function LandingFilm() {
	const video = useRef<HTMLVideoElement>(null);

	useEffect(() => {
		const el = video.current;
		if (!el || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const io = new IntersectionObserver(([entry]) => {
			// play() rejects when the browser refuses autoplay; the controls are there for that.
			if (entry.isIntersecting) el.play().catch(() => {});
			else el.pause();
		}, { threshold: 0.4 });
		io.observe(el);
		return () => io.disconnect();
	}, []);

	return (
		<video
			ref={video}
			src="/film/demo-light.mp4"
			poster="/film/demo-light.jpg"
			width={1200}
			height={750}
			muted
			loop
			playsInline
			controls
			preload="none"
			aria-label="A 30-second film of Rezepte: a recipe scaled to eight servings, edited, cooked with a timer, commented on, passed on as an image, then planned for the week and shopped from a list"
			className="block h-auto w-full rounded-2xl border border-fd-border bg-fd-card shadow-2xl"
		/>
	);
}
