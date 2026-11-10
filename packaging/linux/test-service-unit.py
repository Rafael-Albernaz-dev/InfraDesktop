"""Aceite L-PULSO-PKG: a unit libera ping só ao serviço, sem afrouxar o resto.

Escrito por Claude. Copiar para packaging/linux/test-service-unit.py sem editar.
Rodar: python3 packaging/linux/test-service-unit.py
"""
from pathlib import Path
import unittest

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
UNIT = HERE / 'infradesktopd@.service'
PACKAGE = ROOT / 'scripts/build/package.py'

HARDENING = {
    'User': '%i',
    'NoNewPrivileges': 'true',
    'PrivateTmp': 'true',
    'ProtectSystem': 'strict',
    'ProtectHome': 'true',
    'RestrictAddressFamilies': 'AF_UNIX AF_INET AF_INET6',
    'LockPersonality': 'true',
    'RestrictSUIDSGID': 'true',
    'UMask': '0077',
    'RuntimeDirectoryMode': '0700',
    'StateDirectoryMode': '0700',
}


def service_section():
    values, section = {}, None
    for raw in UNIT.read_text().splitlines():
        line = raw.strip()
        if not line or line.startswith(('#', ';')):
            continue
        if line.startswith('['):
            section = line
            continue
        if section == '[Service]':
            key, _, value = line.partition('=')
            values.setdefault(key.strip(), []).append(value.strip())
    return values


class ServiceUnitTests(unittest.TestCase):
    def setUp(self):
        self.svc = service_section()

    def only(self, key):
        self.assertIn(key, self.svc, f'{key} ausente')
        self.assertEqual(len(self.svc[key]), 1, f'{key} repetido: {self.svc[key]}')
        return self.svc[key][0]

    def test_bounding_set_is_exactly_net_raw(self):
        self.assertEqual(self.only('CapabilityBoundingSet'), 'CAP_NET_RAW')

    def test_ambient_is_exactly_net_raw(self):
        self.assertEqual(self.only('AmbientCapabilities'), 'CAP_NET_RAW')

    def test_no_other_capability_or_privilege_keys(self):
        for key in ('SecureBits', 'PermissionsStartOnly', 'Group', 'SupplementaryGroups'):
            with self.subTest(key=key):
                self.assertNotIn(key, self.svc)
        self.assertNotEqual(self.only('User'), 'root')

    def test_hardening_is_unchanged(self):
        for key, expected in HARDENING.items():
            with self.subTest(key=key):
                self.assertEqual(self.only(key), expected)

    def test_package_ships_no_sysctl_and_no_maintainer_script(self):
        text = PACKAGE.read_text()
        for forbidden in ('sysctl.d', 'ping_group_range', 'postinst', 'setcap'):
            with self.subTest(forbidden=forbidden):
                self.assertNotIn(forbidden, text)


if __name__ == '__main__':
    unittest.main()
