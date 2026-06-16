import { describe, it, expect, vi, afterEach } from 'vitest';
import { copyText } from './clipboard';

describe('copyText', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('resolves true when writeText succeeds', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    expect(await copyText('hello')).toBe(true);
    expect(writeText).toHaveBeenCalledWith('hello');
  });

  it('resolves false (no unhandled rejection) when writeText rejects', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    // Must not throw or reject: the rejection is swallowed internally.
    await expect(copyText('hello')).resolves.toBe(false);
  });

  it('resolves false when the clipboard API is absent', async () => {
    vi.stubGlobal('navigator', {});
    expect(await copyText('hello')).toBe(false);
  });
});
