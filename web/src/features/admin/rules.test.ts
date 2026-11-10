import { describe, expect, it } from 'vitest';
import type { Category, RoutingRule } from '@/shared/api/types';
import { categoriesWithoutRule } from './RulesTab';

const cat = (id: number, name: string, active = true): Category => ({ id, name, description: '', icon: '', sort: id, active });
const rule = (category_id: number, request_type: RoutingRule['request_type'], active = true) =>
  ({ id: category_id * 10, category_id, request_type, active }) as RoutingRule;

describe('categoriesWithoutRule', () => {
  it('lists active categories without an active rule', () => {
    const cats = [cat(1, 'ИТ'), cat(2, 'Другое'), cat(3, 'Архив', false)];
    expect(categoriesWithoutRule(cats, [rule(1, null)])).toEqual(['Другое']);
  });
  it('ignores inactive rules and treats a typed rule as partial coverage', () => {
    const cats = [cat(1, 'ИТ'), cat(2, 'Охрана труда')];
    const out = categoriesWithoutRule(cats, [rule(1, null, false), rule(2, 'problem')]);
    expect(out[0]).toBe('ИТ');
    expect(out[1]).toMatch(/^Охрана труда \(только /);
  });
  it('is empty when every type of every category is covered', () => {
    expect(categoriesWithoutRule([cat(1, 'ИТ')], [rule(1, 'problem'), rule(1, 'idea'), rule(1, 'feedback')])).toEqual([]);
    expect(categoriesWithoutRule([cat(1, 'ИТ')], [rule(1, 'problem'), rule(1, 'idea')])).toEqual(['ИТ (только положительный отзыв)']);
  });
});
