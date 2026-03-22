// DayRow — a single day's entry row (day type selector + hours input).

const DAY_TYPES = [
    { value: 'wfh',            label: 'Work from home' },
    { value: 'part_wfh',       label: 'Part day WFH' },
    { value: 'office',         label: 'Office' },
    { value: 'annual_leave',   label: 'Annual leave' },
    { value: 'sick_leave',     label: 'Sick leave' },
    { value: 'public_holiday', label: 'Public holiday' },
    { value: 'weekend',        label: 'Weekend' },
];

export function renderDayRow(container, { date, entry, onChange }) {
    // TODO: render a table row with:
    //   - date label
    //   - day_type <select> pre-selected to entry.day_type
    //   - hours <input type="number" step="0.25"> shown only for wfh/part_wfh types
    //   - call onChange({ date, day_type, hours }) on change
    container.innerHTML = `<p>${date}</p>`;
}
