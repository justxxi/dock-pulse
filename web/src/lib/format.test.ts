import { describe, it, expect } from 'vitest';
import { formatBytes, formatUptime, truncateId } from './format';

describe('format utilities', () => {
  it('formats bytes correctly', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1048576)).toBe('1.0 MB');
    expect(formatBytes(1073741824)).toBe('1.0 GB');
  });

  it('truncates container id to 12 chars', () => {
    expect(truncateId('abcdef1234567890')).toBe('abcdef123456');
    expect(truncateId('short')).toBe('short');
  });

  it('formats uptime string', () => {
    const past = new Date(Date.now() - 3600 * 1000).toISOString();
    expect(formatUptime(past)).toContain('h');
  });
});
