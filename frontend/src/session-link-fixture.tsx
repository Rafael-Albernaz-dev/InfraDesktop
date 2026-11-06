export function sessionLinkFixture(time: number) {
  return {
    cpe_id: 'CPE-0001',
    ap_id: 'AP-NORTE',
    concentrator_id: 'CCR-CENTRO',
    mac: '02:00:5E:00:00:01',
    interface: 'wlan1',
    bssid: '02:00:5E:00:00:01',
    confirmed_at: new Date(time - 120000).toISOString(),
    confirmed_now: true,
    origin: 'confirmed',
    scope: 'fixture',
    login: 'cliente-0001',
  };
}
