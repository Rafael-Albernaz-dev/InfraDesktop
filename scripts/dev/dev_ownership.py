"""External ownership journal for one DEV invocation; no name/port cleanup."""
from pathlib import Path
import ctypes
import fcntl
import json
import os
import signal
import subprocess
import sys
import time


def identity(pid):
    try:
        fields = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
        return {'pid': pid, 'state': fields[0], 'group': int(fields[2]), 'session': int(fields[3]), 'start': fields[19]}
    except (FileNotFoundError, ProcessLookupError):
        return None


def register(path):
    # The wrapper registers and fsyncs before exec; killing the supervisor cannot
    # leave an exec'ed child outside the external journal.
    item = identity(os.getpid())
    fd = os.open(path, os.O_RDWR | os.O_APPEND | os.O_NOFOLLOW)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX)
        data = os.read(fd, 65536).decode()
        closed = any(json.loads(line).get('closed') for line in data.splitlines())
        os.write(fd, (json.dumps(item) + '\n').encode())
        os.fsync(fd)
    finally:
        os.close(fd)
    if closed:
        raise SystemExit(0)  # external cleanup won; never exec a late child



class Invocation:
    def __init__(self, command, env, log, journal):
        self.journal = journal
        journal.touch(mode=0o600, exist_ok=False)
        self.output = log.open('w')
        self.runner = subprocess.Popen(command, env=dict(env, INFRADESKTOP_DEV_OWNERSHIP=str(journal)),
                                       stdout=self.output, stderr=self.output, start_new_session=True)
        self.owner = identity(self.runner.pid)

    def cleanup(self):
        # Journal is outside supervisor. Validate exact leaders before group signals.
        with self.journal.open('a+') as journal:
            fcntl.flock(journal, fcntl.LOCK_EX)
            journal.write('{"closed":true}\n')
            journal.flush()
            os.fsync(journal.fileno())
            journal.seek(0)
            records = [item for line in journal if (item := json.loads(line)).get('pid') is not None]
        records.append(self.owner)
        observed = {}
        for record in records:
            current = identity(record['pid'])
            if current and (current['start'] != record['start'] or current['group'] != record['pid']):
                continue  # PID reuse is never authority to signal.
            members = []
            for entry in Path('/proc').iterdir():
                if entry.name.isdigit():
                    member = identity(int(entry.name))
                    if member and member['group'] == record['pid']:
                        # A live group reserves its leader PID even after leader
                        # exit. Session + monotonic start bind remaining members.
                        assert member['session'] == record['pid'] and int(member['start']) >= int(record['start'])
                        members.append(member)
                        observed[(member['pid'], member['start'])] = member
            if members:
                try:
                    os.killpg(record['pid'], signal.SIGKILL)
                except ProcessLookupError:
                    pass
        deadline = time.monotonic() + 4
        while time.monotonic() < deadline:
            self.runner.poll()
            while True:
                try:
                    if os.waitpid(-1, os.WNOHANG)[0] == 0:
                        break
                except ChildProcessError:
                    break
            live = [item for (pid, start), item in observed.items()
                    if (current := identity(pid)) and current['start'] == start]
            if not live:
                break
            time.sleep(.025)
        self.output.close()
        assert not live, f'external DEV cleanup deadline: {live}'
        return records, list(observed.values())


def subreaper():
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(36, 1, 0, 0, 0) != 0:
        raise OSError(ctypes.get_errno(), 'external test subreaper unavailable')


if __name__ == '__main__':
    register(sys.argv[1])
    os.execvpe(sys.argv[2], sys.argv[2:], os.environ)
