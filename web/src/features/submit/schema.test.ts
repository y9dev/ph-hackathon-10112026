import { describe, expect, it } from 'vitest';
import { checkFiles, makeSubmitSchema, toPayload } from './schema';

const limits = { max_files: 5, max_file_mb: 10, max_total_mb: 10, allowed_extensions: ['png', 'jpg', 'pdf', 'txt'], title_max: 140, body_max: 4000 };
const schema = makeSubmitSchema(limits);
const valid = { type: 'problem' as const, priority: 'medium' as const, category_id: 1, title: 'Не работает VPN', body: 'VPN на объекте не подключается третий день подряд', proposed_solution: '', is_anonymous: false };

function file(name: string, size: number) {
  const f = new File(['x'], name);
  Object.defineProperty(f, 'size', { value: size });
  return f;
}

describe('submit schema (same limits as the server)', () => {
  it('accepts a valid request', () => {
    expect(schema.safeParse(valid).success).toBe(true);
  });
  it('requires a category', () => {
    const r = schema.safeParse({ ...valid, category_id: undefined });
    expect(r.success).toBe(false);
    expect(r.error?.issues[0].message).toBe('Выберите категорию');
  });
  it('enforces title 5..140 and body 20..4000 after trimming', () => {
    expect(schema.safeParse({ ...valid, title: '  abc  ' }).success).toBe(false);
    expect(schema.safeParse({ ...valid, title: 'x'.repeat(141) }).success).toBe(false);
    expect(schema.safeParse({ ...valid, body: 'короткий текст' }).success).toBe(false);
    expect(schema.safeParse({ ...valid, body: 'x'.repeat(4001) }).success).toBe(false);
  });
  it('limits the proposed solution to 2000', () => {
    expect(schema.safeParse({ ...valid, proposed_solution: 'x'.repeat(2001) }).success).toBe(false);
  });
  it('builds the CreateRequest payload without empty optional fields', () => {
    expect(toPayload({ ...valid, title: '  Не работает VPN ' })).toEqual({
      type: 'problem',
      category_id: 1,
      title: 'Не работает VPN',
      body: valid.body,
      is_anonymous: false,
      priority: 'medium',
    });
  });
  it('accepts positive feedback and every priority, rejects unknown ones', () => {
    for (const priority of ['low', 'medium', 'high'] as const) expect(schema.safeParse({ ...valid, type: 'feedback', priority }).success).toBe(true);
    expect(schema.safeParse({ ...valid, priority: 'urgent' }).success).toBe(false);
    expect(schema.safeParse({ ...valid, type: 'praise' }).success).toBe(false);
  });
  it('drops the proposed solution for feedback (the form hides that field)', () => {
    expect(toPayload({ ...valid, type: 'feedback', proposed_solution: 'старый текст' })).not.toHaveProperty('proposed_solution');
  });
});

describe('checkFiles', () => {
  it('rejects bad types and oversized files with a reason', () => {
    const r = checkFiles([], [file('a.exe', 10), file('b.pdf', 11 * 1024 * 1024), file('c.png', 1000)], limits);
    expect(r.accepted.map((f) => f.name)).toEqual(['c.png']);
    expect(r.rejected).toEqual([
      { name: 'a.exe', reason: 'Такой формат не поддерживается' },
      { name: 'b.pdf', reason: 'Файл больше 10 МБ' },
    ]);
  });
  it('enforces the file count and total size', () => {
    const five = [1, 2, 3, 4, 5].map((i) => file(`${i}.txt`, 10));
    expect(checkFiles(five, [file('6.txt', 10)], limits).rejected[0].reason).toBe('Можно прикрепить не больше 5 файлов');
    const big = [file('a.pdf', 6 * 1024 * 1024)];
    expect(checkFiles(big, [file('b.pdf', 5 * 1024 * 1024)], limits).rejected[0].reason).toBe('Общий размер файлов больше 10 МБ');
  });
  it('ignores exact duplicates', () => {
    const a = file('a.txt', 10);
    expect(checkFiles([a], [file('a.txt', 10)], limits).accepted).toHaveLength(1);
  });
});
