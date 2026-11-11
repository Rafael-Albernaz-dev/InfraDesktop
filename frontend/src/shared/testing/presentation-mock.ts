import { vi } from 'vitest';
// Unconfigured capabilities fail explicitly; adapter-boundary journeys may retain real implementations.
function mockMethod(implementation?: unknown) {
  return vi.fn(async (...args: unknown[]) =>
    typeof implementation === 'function'
      ? implementation(...args)
      : { code: 'SERVICE_UNAVAILABLE' },
  );
}

const accessMock = (fallback: Record<string, unknown> = {}) => ({
  status: mockMethod(fallback.status),
  register: mockMethod(fallback.register),
  login: mockMethod(fallback.login),
  recover: mockMethod(fallback.recover),
  resetAccount: mockMethod(fallback.resetAccount),
  changePassword: mockMethod(fallback.changePassword),
  logout: mockMethod(fallback.logout),
  createCompany: mockMethod(fallback.createCompany),
  selectCompany: mockMethod(fallback.selectCompany),
  setTheme: mockMethod(fallback.setTheme),
  setColumns: mockMethod(fallback.setColumns),
  renewRecovery: mockMethod(fallback.renewRecovery),
  copyRecovery: mockMethod(fallback.copyRecovery),
  ackRecovery: mockMethod(fallback.ackRecovery),
  saveRecovery: mockMethod(fallback.saveRecovery),
  getBrand: mockMethod(fallback.getBrand),
  saveBrand: mockMethod(fallback.saveBrand),
});
const equipmentAPIMock = (fallback: Record<string, unknown> = {}) => ({
  enrollmentStatus: mockMethod(fallback.enrollmentStatus),
  get: mockMethod(fallback.get),
  save: mockMethod(fallback.save),
  active: mockMethod(fallback.active),
  credentials: mockMethod(fallback.credentials),
  deleteCredential: mockMethod(fallback.deleteCredential),
  setDefault: mockMethod(fallback.setDefault),
  putCredential: mockMethod(fallback.putCredential),
  inspect: mockMethod(fallback.inspect),
  confirm: mockMethod(fallback.confirm),
  resetTrust: mockMethod(fallback.resetTrust),
  start: mockMethod(fallback.start),
  status: mockMethod(fallback.status),
  cancel: mockMethod(fallback.cancel),
  list: mockMethod(fallback.list),
  bulk: mockMethod(fallback.bulk),
});
const inventoryImportAPIMock = (fallback: Record<string, unknown> = {}) => ({
  choose: mockMethod(fallback.choose),
  review: mockMethod(fallback.review),
  confirm: mockMethod(fallback.confirm),
  template: mockMethod(fallback.template),
});
const pppoeDiagnosticAPIMock = (fallback: Record<string, unknown> = {}) => ({
  catalog: mockMethod(fallback.catalog),
  radius: mockMethod(fallback.radius),
  ping: mockMethod(fallback.ping),
  status: mockMethod(fallback.status),
  cancel: mockMethod(fallback.cancel),
  current: mockMethod(fallback.current),
  list: mockMethod(fallback.list),
});
const pppoeAPIMock = (fallback: Record<string, unknown> = {}) => ({
  start: mockMethod(fallback.start),
  status: mockMethod(fallback.status),
  cancel: mockMethod(fallback.cancel),
  current: mockMethod(fallback.current),
  observation: mockMethod(fallback.observation),
  list: mockMethod(fallback.list),
  detail: mockMethod(fallback.detail),
  groups: mockMethod(fallback.groups),
  save: mockMethod(fallback.save),
  references: mockMethod(fallback.references),
  remove: mockMethod(fallback.remove),
  compare: mockMethod(fallback.compare),
  export: mockMethod(fallback.export),
  exportOperational: mockMethod(fallback.exportOperational),
});
const pppoeReadAPIMock = (fallback: Record<string, unknown> = {}) => ({
  start: mockMethod(fallback.start),
  stop: mockMethod(fallback.stop),
  status: mockMethod(fallback.status),
  cancel: mockMethod(fallback.cancel),
  cancelOperation: mockMethod(fallback.cancelOperation),
  page: mockMethod(fallback.page),
});
const deviceMonitoringAPIMock = (fallback: Record<string, unknown> = {}) => ({
  current: mockMethod(fallback.current),
});
const radioAPIMock = (fallback: Record<string, unknown> = {}) => ({
  indexPage: mockMethod(fallback.indexPage),
  current: mockMethod(fallback.current),
  start: mockMethod(fallback.start),
  status: mockMethod(fallback.status),
  cancel: mockMethod(fallback.cancel),
  cancelOperation: mockMethod(fallback.cancelOperation),
  confirmFamily: mockMethod(fallback.confirmFamily),
  link: mockMethod(fallback.link),
  confirmLink: mockMethod(fallback.confirmLink),
});

export function presentationMock(
  overrides: Record<string, Record<string, unknown>> = {},
  fallback: Record<string, Record<string, unknown>> = {},
) {
  return {
    access: { ...accessMock(fallback.access), ...overrides.access },
    equipmentAPI: {
      ...equipmentAPIMock(fallback.equipmentAPI),
      ...overrides.equipmentAPI,
    },
    inventoryImportAPI: {
      ...inventoryImportAPIMock(fallback.inventoryImportAPI),
      ...overrides.inventoryImportAPI,
    },
    pppoeDiagnosticAPI: {
      ...pppoeDiagnosticAPIMock(fallback.pppoeDiagnosticAPI),
      ...overrides.pppoeDiagnosticAPI,
    },
    pppoeAPI: { ...pppoeAPIMock(fallback.pppoeAPI), ...overrides.pppoeAPI },
    pppoeReadAPI: {
      ...pppoeReadAPIMock(fallback.pppoeReadAPI),
      ...overrides.pppoeReadAPI,
    },
    deviceMonitoringAPI: {
      ...deviceMonitoringAPIMock(fallback.deviceMonitoringAPI),
      ...overrides.deviceMonitoringAPI,
    },
    radioAPI: { ...radioAPIMock(fallback.radioAPI), ...overrides.radioAPI },
  };
}
