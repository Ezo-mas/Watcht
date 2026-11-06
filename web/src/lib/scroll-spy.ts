// Which section of a title page is in view, for the section list's
// highlight (TitlePage).

export interface SectionTop {
	id: string;
	/** The section's top edge, relative to the top of the window. */
	top: number;
}

/**
 * The section in view: the last one whose top has scrolled above `line`
 * (a little below the header), or the first when none has. At the bottom
 * of the page, short sections at the end can never reach the line, so the
 * last one showing wins.
 */
export function currentSection(
	tops: SectionTop[],
	line: number,
	atBottom: boolean,
	viewport: number
): string | undefined {
	if (tops.length === 0) return undefined;
	if (atBottom) {
		for (let i = tops.length - 1; i >= 0; i--) {
			if (tops[i].top < viewport) return tops[i].id;
		}
	}
	let current = tops[0].id;
	for (const s of tops) {
		if (s.top <= line) current = s.id;
	}
	return current;
}
