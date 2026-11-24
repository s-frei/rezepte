// Encodes every film the last `film/` run recorded (test-results/film-frames/
// <name>/, see film/film.ts) into public/film/<name>.mp4 for the landing, and
// its first frame as public/film/<name>.jpg, the video's poster. The film ends
// in a 400 ms crossfade into that frame, so the loop has no cut. ffmpeg comes from
// docs/user/mise.toml's [tools].
import { execFileSync } from 'node:child_process';
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const FRAMES = fileURLToPath(new URL('../test-results/film-frames', import.meta.url));
const OUT = fileURLToPath(new URL('../public/film', import.meta.url));
const ffmpeg = (...args: string[]) => execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args], { stdio: 'inherit' });

const FADE = 0.4;

const names = existsSync(FRAMES) ? readdirSync(FRAMES).filter((n) => existsSync(path.join(FRAMES, n, 'frames.txt'))) : [];
if (!names.length) throw new Error(`no film in ${FRAMES}: run mise run //docs/user:film`);
mkdirSync(OUT, { recursive: true });

for (const name of names) {
	const dir = path.join(FRAMES, name);
	const marks: Record<string, number> = JSON.parse(readFileSync(path.join(dir, 'marks.json'), 'utf8'));
	// Input 0 is the film, input 1 its first frame held for the crossfade.
	const input = ['-f', 'concat', '-safe', '0', '-i', path.join(dir, 'frames.txt'), '-loop', '1', '-framerate', '30', '-t', String(FADE), '-i', path.join(dir, '00000.jpg')];
	/** The whole film, its last FADE seconds blended into the first frame. */
	const looped =
		`[0:v]fps=30,trim=end=${marks.duration},setpts=PTS-STARTPTS,format=yuv420p,setsar=1[a];` +
		`[1:v]fps=30,format=yuv420p,setsar=1[b];` +
		`[a][b]xfade=transition=fade:duration=${FADE}:offset=${(marks.duration - FADE).toFixed(3)}`;
	const out = { mp4: path.join(OUT, `${name}.mp4`), poster: path.join(OUT, `${name}.jpg`) };
	// xfade renegotiates the pixel format, so without the final scale and format the MP4
	// comes out as full-range 4:4:4 (High 4:4:4 profile), which Safari refuses as damaged.
	ffmpeg(...input, '-filter_complex', `${looped},scale=out_range=tv,format=yuv420p[v]`, '-map', '[v]', '-c:v', 'libx264', '-preset', 'slow', '-crf', '22', '-tune', 'animation', '-movflags', '+faststart', '-an', out.mp4);
	copyFileSync(path.join(dir, '00000.jpg'), out.poster);
	for (const f of Object.values(out)) console.log(`${path.relative(process.cwd(), f)}  ${(statSync(f).size / 1024 / 1024).toFixed(2)} MB`);
}
