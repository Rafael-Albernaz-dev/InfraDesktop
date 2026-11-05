import type { Result } from './presentation';

export function operationalAccount(): Result {
  return {
    code: 'OK',
    request_id: 't',
    instance_id: 'one',
    activation_id: 'a',
    profile_id: 'X',
    heartbeat: 1,
    setup: false,
    authenticated: true,
    name: 'reader',
    theme: 'light',
    recovery: '',
    companies: [{ id: 'X', name: 'Company X' }],
  };
}
