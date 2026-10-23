import { Stack, TextField } from '@mui/material';
import { useState } from 'react';
import { isoDate, minutesToClock, parseClock } from '../lib/format';

// Native time and datetime-local inputs follow the browser locale and may show AM/PM,
// so times are typed as HH:MM on a 24h clock instead.

interface ClockProps {
  label: string;
  /** Minutes since midnight; null for no time. */
  minutes: number | null;
  onChange: (min: number | null) => void;
  /** An empty field is valid and reports null. */
  optional?: boolean;
}

export function ClockField({ label, minutes, onChange, optional }: ClockProps) {
  const [draft, setDraft] = useState(() => (minutes === null ? '' : minutesToClock(minutes)));
  const parsed = parseClock(draft);
  // Follow outside changes without reformatting what is being typed.
  const [seen, setSeen] = useState(minutes);
  if (minutes !== seen) {
    setSeen(minutes);
    if (minutes !== parsed) setDraft(minutes === null ? '' : minutesToClock(minutes));
  }
  const blank = !draft.trim();
  return (
    <TextField
      label={label}
      value={draft}
      error={parsed === null && !(optional && blank)}
      placeholder="09:00"
      onChange={(e) => {
        const v = e.target.value;
        setDraft(v);
        const m = parseClock(v);
        if (m !== null) onChange(m);
        else if (optional && !v.trim()) onChange(null);
      }}
      onBlur={() => parsed !== null && setDraft(minutesToClock(parsed))}
      slotProps={{ htmlInput: { inputMode: 'numeric', maxLength: 5 }, inputLabel: { shrink: true } }}
      sx={{ width: 120, flexShrink: 0 }}
    />
  );
}

interface DateTimeProps {
  label: string;
  /** Local date and time as YYYY-MM-DDTHH:MM (the datetime-local format); '' for none. */
  value: string;
  onChange: (value: string) => void;
  helperText?: string;
}

export function DateTimeField({ label, value, onChange, helperText }: DateTimeProps) {
  const [date, time = ''] = value ? value.split('T') : [''];
  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: 'flex-start' }}>
      <TextField
        type="date"
        label={label}
        value={date}
        onChange={(e) => onChange(e.target.value ? `${e.target.value}T${time || '09:00'}` : '')}
        helperText={helperText}
        slotProps={{ inputLabel: { shrink: true } }}
        sx={{ flex: 1 }}
      />
      <ClockField
        label="Time"
        optional
        minutes={time ? parseClock(time) : null}
        onChange={(m) => onChange(m === null ? '' : `${date || isoDate(new Date())}T${minutesToClock(m)}`)}
      />
    </Stack>
  );
}
