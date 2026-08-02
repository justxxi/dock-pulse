export interface AnsiSpan {
  text: string;
  color?: string;
  bgColor?: string;
  bold?: boolean;
  dim?: boolean;
  italic?: boolean;
  underline?: boolean;
}

const colorMap: Record<number, string> = {
  30: '#1e293b',
  31: '#ef4444',
  32: '#10b981',
  33: '#f59e0b',
  34: '#3b82f6',
  35: '#a855f7',
  36: '#06b6d4',
  37: '#f1f5f9',
  90: '#64748b',
  91: '#f87171',
  92: '#34d399',
  93: '#fbbf24',
  94: '#60a5fa',
  95: '#c084fc',
  96: '#22d3ee',
  97: '#ffffff',
};

export function parseAnsi(input: string): AnsiSpan[] {
  const spans: AnsiSpan[] = [];
  const regex = /\x1b\[([0-9;]*)m/g;

  let lastIndex = 0;
  let currentSpan: AnsiSpan = { text: '' };

  let match: RegExpExecArray | null;
  while ((match = regex.exec(input)) !== null) {
    const textChunk = input.slice(lastIndex, match.index);
    if (textChunk.length > 0) {
      spans.push({ ...currentSpan, text: textChunk });
    }

    lastIndex = regex.lastIndex;
    const codes = (match[1] || '0').split(';').map(n => parseInt(n, 10) || 0);

    for (const code of codes) {
      if (code === 0) {
        currentSpan = { text: '' };
      } else if (code === 1) {
        currentSpan.bold = true;
      } else if (code === 2) {
        currentSpan.dim = true;
      } else if (code === 3) {
        currentSpan.italic = true;
      } else if (code === 4) {
        currentSpan.underline = true;
      } else if (colorMap[code]) {
        currentSpan.color = colorMap[code];
      }
    }
  }

  const remaining = input.slice(lastIndex);
  if (remaining.length > 0) {
    spans.push({ ...currentSpan, text: remaining });
  }

  return spans;
}

export function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

export function ansiToHtml(input: string): string {
  const spans = parseAnsi(input);
  return spans.map(span => {
    const styles: string[] = [];
    if (span.color) styles.push(`color:${span.color}`);
    if (span.bold) styles.push('font-weight:bold');
    if (span.dim) styles.push('opacity:0.6');
    if (span.italic) styles.push('font-style:italic');
    if (span.underline) styles.push('text-decoration:underline');

    const escapedText = escapeHtml(span.text);
    if (styles.length > 0) {
      return `<span style="${styles.join(';')}">${escapedText}</span>`;
    }
    return escapedText;
  }).join('');
}
