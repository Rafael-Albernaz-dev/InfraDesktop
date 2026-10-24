#!/usr/bin/env python3
"""Build and inspect the minimum Debian package without installing it."""
from pathlib import Path
import os
import shutil
import subprocess

root = Path(__file__).resolve().parents[1]
work = root / 'build/package-bf04'
# Dedicated owned build tree only, never a global cleanup.
if work.exists():
    shutil.rmtree(work)
stage = work / 'root'
for binary, target in [('infradesktop', 'usr/bin/infradesktop'), ('infradesktopd', 'usr/lib/infradesktop/infradesktopd')]:
    dest = stage / target
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(root / 'build/bin' / binary, dest)
    dest.chmod(0o755)
for source, target, mode in [
    ('infradesktopd@.service', 'usr/lib/systemd/system/infradesktopd@.service', 0o644),
    ('infradesktop.desktop', 'usr/share/applications/infradesktop.desktop', 0o644),
    ('provision-technician', 'usr/lib/infradesktop/provision-technician', 0o755),
    ('provision-vault', 'usr/lib/infradesktop/provision-vault', 0o755),
    ('infradesktop-vault.conf', 'usr/share/infradesktop/infradesktop-vault.conf', 0o644),
]:
    dest = stage / target
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(root / 'packaging/linux' / source, dest)
    dest.chmod(mode)
for size in [512,256,128,64,48,32]:
    dest=stage/f'usr/share/icons/hicolor/{size}x{size}/apps/infradesktop.png'
    dest.parent.mkdir(parents=True,exist_ok=True)
    shutil.copy2(root/f'packaging/linux/assets/infradesktop-{size}.png',dest);dest.chmod(0o644)
dest=stage/'usr/share/icons/hicolor/scalable/apps/infradesktop.svg';dest.parent.mkdir(parents=True,exist_ok=True)
shutil.copy2(root/'packaging/linux/assets/infradesktop.svg',dest);dest.chmod(0o644)
for source in [root/'packaging/linux/assets/ORIGIN.md', *sorted((root/'frontend/public/fonts').glob('*LICENSE.txt'))]:
    dest=stage/'usr/share/doc/infradesktop'/source.name;dest.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(source,dest);dest.chmod(0o644)
licenses=root/'packaging/linux/licenses'
if licenses.exists():
    license_dest=stage/'usr/share/doc/infradesktop/licenses'
    shutil.copytree(licenses,license_dest)
    for item in license_dest.rglob('*'):
        if item.is_file():item.chmod(0o644)
(work / 'debian').mkdir(parents=True)
(work / 'debian/control').write_text('Source: infradesktop\nSection: net\nPriority: optional\nMaintainer: InfraDesktop <noreply@localhost>\nStandards-Version: 4.6.2\n\nPackage: infradesktop\nArchitecture: amd64\nDescription: Local foundation for ISP technicians\n')
for binary in ['infradesktop', 'infradesktopd']:
    subprocess.run(['ldd', str(root / 'build/bin' / binary)], check=True)
deps = subprocess.check_output(['dpkg-shlibdeps', '-O', '-S' + str(stage), '-e' + str(stage / 'usr/bin/infradesktop'), '-e' + str(stage / 'usr/lib/infradesktop/infradesktopd')], cwd=work, text=True).strip()
if not deps.startswith('shlibs:Depends='):
    raise RuntimeError('Could not determine runtime dependencies')
deps = deps.split('=', 1)[1]
(stage / 'DEBIAN').mkdir()
(stage / 'DEBIAN/control').write_text(f'Package: infradesktop\nVersion: 0.6.0\nSection: net\nPriority: optional\nArchitecture: amd64\nMaintainer: InfraDesktop <noreply@localhost>\nDepends: {deps}, systemd\nDescription: Local ISP workspace with local account and persistence\n Local account, company inventory, encrypted credentials and on-demand SSH reads.\n')
# Verify a named non-root instantiated unit against the actual staged binary.
unit = (root / 'packaging/linux/infradesktopd@.service').read_text().replace('/usr/lib/infradesktop/infradesktopd', str(root / 'build/bin/infradesktopd'))
verify = work / 'infradesktopd@.service'
verify.write_text(unit)
verify_env = dict(os.environ, SYSTEMD_UNIT_PATH=str(work) + ':/usr/lib/systemd/system')
subprocess.run(['systemd-analyze', 'verify', str(verify)], check=True, env=verify_env)
for directory in [stage, *[p for p in stage.rglob('*') if p.is_dir()]]:
    directory.chmod(0o755)
output = root / 'build/infradesktop_0.6.0_amd64.deb'
subprocess.run(['dpkg-deb', '--root-owner-group', '--build', str(stage), str(output)], check=True)
for flag in ['--info', '--contents']:
    subprocess.run(['dpkg-deb', flag, str(output)], check=True)
print('Package inspection PASS; NOT INSTALLED; service NOT ENABLED/STARTED')
