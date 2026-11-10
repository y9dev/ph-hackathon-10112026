/** Confidence of a suggested group, derived from its similarity score (0..1). */
export type Confidence = 'high' | 'medium' | 'low';

export function confidenceOf(score: number): Confidence {
  if (score >= 0.5) return 'high';
  if (score >= 0.35) return 'medium';
  return 'low';
}
