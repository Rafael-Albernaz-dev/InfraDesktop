"""Synthetic helper argument validation; no privileged calls or system writes."""
import contextlib
import io
import pwd
import stat
import tempfile
from types import SimpleNamespace
from xml.etree import ElementTree
import os
from pathlib import Path
import runpy
import sys
import unittest
from unittest.mock import patch

HELPER = Path(__file__).with_name('reset-account')


class ResetHelperTests(unittest.TestCase):
    def test_missing_pkexec_uid_fails_before_runtime_access(self):
        module = runpy.run_path(str(HELPER))
        with patch.object(sys, 'argv', [str(HELPER), 'synthetic', 'a' * 64]), \
             patch.dict(os.environ, {}, clear=True), \
             patch.object(os, 'geteuid', return_value=0), \
             patch.object(os, 'open', side_effect=AssertionError('runtime accessed')), \
             contextlib.redirect_stderr(io.StringIO()):
            with self.assertRaisesRegex(SystemExit, 'Administrative authorization required'):
                module['main']()

    def test_invalid_identity_and_nonce_fail_before_runtime_access(self):
        module = runpy.run_path(str(HELPER))
        for name, nonce in [('../root', 'a' * 64), ('synthetic', 'bad')]:
            with self.subTest(name=name, nonce=nonce), \
                 patch.object(sys, 'argv', [str(HELPER), name, nonce]), \
                 patch.object(os, 'open', side_effect=AssertionError('runtime accessed')):
                with self.assertRaisesRegex(SystemExit, 'Invalid request'):
                    module['main']()


    def test_pkexec_user_mismatch_and_root_are_refused(self):
        module = runpy.run_path(str(HELPER))
        for info in [SimpleNamespace(pw_uid=0, pw_name='synthetic'),
                     SimpleNamespace(pw_uid=1000, pw_name='different')]:
            with self.subTest(info=info), \
                 patch.object(sys, 'argv', [str(HELPER), 'synthetic', 'a' * 64]), \
                 patch.dict(os.environ, {'PKEXEC_UID': '1000'}, clear=True), \
                 patch.object(os, 'geteuid', return_value=0), \
                 patch.object(pwd, 'getpwuid', return_value=info), \
                 patch.object(os, 'open', side_effect=AssertionError('runtime accessed')):
                with self.assertRaisesRegex(SystemExit, 'Reset authorization failed'):
                    module['main']()

    def test_publishes_nonce_in_isolated_runtime_without_following_links(self):
        module = runpy.run_path(str(HELPER))
        uid = os.getuid()
        if uid == 0:
            self.fail('fixture must run unprivileged')
        with tempfile.TemporaryDirectory() as root:
            runtime = Path(root) / 'infradesktop-synthetic'
            runtime.mkdir(mode=0o700)
            real_open, real_fstat = os.open, os.fstat
            runfds = []
            def open_fixture(path, flags, mode=0o777, *, dir_fd=None):
                fd = real_open(root if path == '/run' else path, flags, mode, dir_fd=dir_fd)
                if path == '/run':
                    runfds.append(fd)
                return fd
            def stat_fixture(fd):
                if fd in runfds:
                    return SimpleNamespace(st_uid=0, st_mode=stat.S_IFDIR | 0o755)
                return real_fstat(fd)
            with patch.object(sys, 'argv', [str(HELPER), 'synthetic', 'a' * 64]), \
                 patch.dict(os.environ, {'PKEXEC_UID': str(uid)}, clear=True), \
                 patch.object(os, 'geteuid', return_value=0), \
                 patch.object(pwd, 'getpwuid', return_value=SimpleNamespace(pw_uid=uid, pw_name='synthetic')), \
                 patch.object(os, 'open', side_effect=open_fixture), \
                 patch.object(os, 'fstat', side_effect=stat_fixture), \
                 patch.object(os, 'fchown') as chown:
                module['main']()
                self.assertEqual((runtime / ('reset-' + 'a' * 64)).read_bytes(), b'a' * 64)
                self.assertEqual(stat.S_IMODE((runtime / ('reset-' + 'a' * 64)).stat().st_mode), 0o644)
                self.assertEqual(chown.call_args.args[1:], (0, 0))

    def test_rejects_symlink_as_runtime_directory(self):
        module = runpy.run_path(str(HELPER))
        uid = os.getuid()
        if uid == 0:
            self.fail('fixture must run unprivileged')
        with tempfile.TemporaryDirectory() as root:
            target = Path(root) / 'target'
            target.mkdir(mode=0o700)
            (Path(root) / 'infradesktop-synthetic').symlink_to(target, target_is_directory=True)
            real_open, real_fstat = os.open, os.fstat
            runfds = []
            def open_fixture(path, flags, mode=0o777, *, dir_fd=None):
                fd = real_open(root if path == '/run' else path, flags, mode, dir_fd=dir_fd)
                if path == '/run':
                    runfds.append(fd)
                return fd
            def stat_fixture(fd):
                if fd in runfds:
                    return SimpleNamespace(st_uid=0, st_mode=stat.S_IFDIR | 0o755)
                return real_fstat(fd)
            with patch.object(sys, 'argv', [str(HELPER), 'synthetic', 'b' * 64]), \
                 patch.dict(os.environ, {'PKEXEC_UID': str(uid)}, clear=True), \
                 patch.object(os, 'geteuid', return_value=0), \
                 patch.object(pwd, 'getpwuid', return_value=SimpleNamespace(pw_uid=uid, pw_name='synthetic')), \
                 patch.object(os, 'open', side_effect=open_fixture), \
                 patch.object(os, 'fstat', side_effect=stat_fixture):
                with self.assertRaisesRegex(SystemExit, 'Reset authorization failed'):
                    module['main']()
            self.assertEqual(list(target.iterdir()), [])

    def test_package_policy_requires_uncached_admin_and_lists_helper(self):
        action = ElementTree.parse(HELPER.with_name('org.infradesktop.reset-account.policy')).getroot().find('action')
        self.assertIsNotNone(action)
        self.assertEqual([node.text for node in action.find('defaults')], ['auth_admin'] * 3)
        self.assertEqual(action.find('annotate').text, '/usr/lib/infradesktop/reset-account')
        package = (HELPER.parents[2] / 'scripts/build/package.py').read_text()
        self.assertIn("'reset-account', 'usr/lib/infradesktop/reset-account'", package)
        self.assertIn('usr/share/polkit-1/actions/org.infradesktop.reset-account.policy', package)


if __name__ == '__main__':
    unittest.main()
