import { describe, expect, it } from 'vitest';
import { confidenceOf } from './confidence';

describe('confidenceOf', () => {
  it('maps the similarity score to high / medium / low', () => {
    expect(confidenceOf(0.9)).toBe('high');
    expect(confidenceOf(0.5)).toBe('high');
    expect(confidenceOf(0.49)).toBe('medium');
    expect(confidenceOf(0.35)).toBe('medium');
    expect(confidenceOf(0.349)).toBe('low');
    expect(confidenceOf(0)).toBe('low');
  });
});
