// WeekNav — previous/next week navigation component.

export function renderWeekNav(container, { weekStart, onPrev, onNext }) {
    // TODO: render week label (e.g. "17 Mar – 23 Mar 2025") with prev/next buttons
    container.innerHTML = `<p>Week of ${weekStart}</p>`;
}
