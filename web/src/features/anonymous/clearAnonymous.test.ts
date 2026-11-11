import { describe, expect, it } from 'vitest';
import { clearAnonymousRequests, loadAnonymousRequests, saveAnonymousRequest } from './deviceStore';

const entry = {
  id: 1,
  number: 'MV-0001',
  title: 'Тест',
  token: 'ABCD-EFGH-JKMN',
  created_at: '2026-11-11T10:00:00Z',
};

describe('clearAnonymousRequests', () => {
  it('removes every saved anonymous request from this device', () => {
    saveAnonymousRequest(entry);
    saveAnonymousRequest({ ...entry, id: 2, number: 'MV-0002', token: 'PQRS-TVWX-YZ23' });
    expect(loadAnonymousRequests()).toHaveLength(2);

    clearAnonymousRequests();

    expect(loadAnonymousRequests()).toEqual([]);
  });

  it('does not throw when nothing is stored', () => {
    expect(() => clearAnonymousRequests()).not.toThrow();
  });
});
