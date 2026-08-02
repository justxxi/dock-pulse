import { describe, it, expect } from 'vitest';
import { parseAnsi, escapeHtml, ansiToHtml } from './ansi';

describe('ansi parser', () => {
  it('parses plain text without ansi codes', () => {
    const result = parseAnsi('hello world');
    expect(result).toEqual([{ text: 'hello world' }]);
  });

  it('parses color codes', () => {
    const result = parseAnsi('\x1b[31mred text\x1b[0m');
    expect(result.length).toBeGreaterThan(0);
    expect(result[0]?.color).toBe('#ef4444');
    expect(result[0]?.text).toBe('red text');
  });

  it('escapes html tags properly', () => {
    const escaped = escapeHtml('<script>alert("xss")</script>');
    expect(escaped).toBe('&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;');
  });

  it('converts ansi to html safely', () => {
    const html = ansiToHtml('\x1b[32m<safe>\x1b[0m');
    expect(html).toContain('&lt;safe&gt;');
    expect(html).toContain('color:#10b981');
  });
});
