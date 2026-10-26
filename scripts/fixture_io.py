"""Bounded framing for owned native-test fixtures; never reads human processes."""
import json
import os
import selectors
import subprocess
import time

# Five seconds permits process startup on a loaded local test workstation. This
# is a harness deadline, independent of SSH/product budgets and not a product SLA.
FIXTURE_DEADLINE = 5
MAX_LINE = 8192


class FixtureOutput:
    def __init__(self, process, budget=FIXTURE_DEADLINE):
        self.process = process
        self.budget = budget
        self.buffer = bytearray()

    def json_line(self, phase):
        deadline = time.monotonic() + self.budget
        with selectors.DefaultSelector() as ready:
            ready.register(self.process.stdout, selectors.EVENT_READ)
            while True:
                newline = self.buffer.find(b'\n')
                if newline > MAX_LINE:
                    raise AssertionError(phase + ': fixture line exceeds framing limit')
                if newline >= 0:
                    line = bytes(self.buffer[:newline])
                    del self.buffer[:newline + 1]
                    return json.loads(line)
                if len(self.buffer) > MAX_LINE:
                    raise AssertionError(phase + ': fixture line exceeds framing limit')
                remaining = deadline - time.monotonic()
                if remaining <= 0 or not ready.select(remaining):
                    state = 'partial line' if self.buffer else 'silence'
                    raise AssertionError(phase + ': fixture deadline (' + state + ')')
                block = os.read(self.process.stdout.fileno(), min(4096, MAX_LINE + 1 - len(self.buffer)))
                if not block:
                    state = 'partial line' if self.buffer else 'EOF'
                    try:
                        code = self.process.wait(timeout=max(0, deadline-time.monotonic()))
                        child = f'child exit={code}'
                    except subprocess.TimeoutExpired:
                        child = 'child alive with closed stdout'
                    raise AssertionError(f'{phase}: fixture {state}; {child}')
                self.buffer.extend(block)

    def finish(self):
        self.process.terminate()
        try:
            counters = self.json_line('final counters')
            self.process.wait(timeout=self.budget)
            if self.process.returncode != 0:
                raise AssertionError('fixture final exit=' + str(self.process.returncode))
            return counters
        finally:
            if self.process.poll() is None:
                self.process.kill()
                self.process.wait(timeout=self.budget)
